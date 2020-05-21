package mergecmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/cmd/common"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
	common2 "gitlab.com/makeos/mosdef/remote/types/common"
	"gitlab.com/makeos/mosdef/util"
)

type MergeRequestCreateArgs struct {
	// IssueID is the unique merge request ID
	MergeRequestNumber int

	// Title is the title of the merge request
	Title string

	// Body is the merge request body
	Body string

	// ReplyHash is the hash of a comment commit
	ReplyHash string

	// Reactions adds or removes reactions to/from a comment commit
	// Negated reactions indicate removal request
	Reactions []string

	// Base is the base branch name
	Base string

	// BaseHash is hash of the base branch
	BaseHash string

	// Target is the target branch name
	Target string

	// TargetHash is the target hash name
	TargetHash string

	// UseEditor indicates that the body of the Issue should be collected using a text editor.
	UseEditor bool

	// EditorPath indicates the path to an editor program
	EditorPath string

	// NoBody prevents prompting user for comment body
	NoBody bool

	// Close sets close status to 1.
	Close *bool

	// Open sets close status to 0
	Open bool

	// StdOut receives the output
	StdOut io.Writer

	// StdIn receives input
	StdIn io.ReadCloser

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// EditorReader is used to read from an editor program
	EditorReader util.EditorReaderFunc

	// InputReader is a function that reads input from stdin
	InputReader util.InputReader
}

// MergeRequestCreateCmd create a new merge request or adds a comment to an existing one
func MergeRequestCreateCmd(r types.LocalRepo, args *MergeRequestCreateArgs) error {

	var nComments int
	var mrRef, mrRefHash string
	var err error

	// If merge-request number is not set, it means a new merge request should be created.
	// Go directly to input collection.
	if args.MergeRequestNumber == 0 {
		goto input
	}

	// Get the merge request reference name and attempt to get it
	mrRef = plumbing.MakeMergeRequestReference(args.MergeRequestNumber)
	mrRefHash, err = r.RefGet(mrRef)

	// When the merge request does not exist and this is a reply intent, return error
	if err != nil && err == plumbing.ErrRefNotFound && args.ReplyHash != "" {
		return fmt.Errorf("merge request (%d) was not found", args.MergeRequestNumber)
	} else if err != nil && err != plumbing.ErrRefNotFound {
		return err
	}

	// Get the number of comments
	nComments, err = r.NumCommits(mrRef, false)
	if err != nil {
		return errors.Wrap(err, "failed to count comments in merge request")
	}

	// Title is not required when the intent is to add a comment
	if nComments > 0 && args.Title != "" {
		return fmt.Errorf("title not required when adding a comment to a merge request")
	}

	// When the intent is to reply to a specific comment, ensure the target
	// comment hash exist in the merge request
	if args.ReplyHash != "" && r.IsAncestor(args.ReplyHash, mrRefHash) != nil {
		return fmt.Errorf("target comment hash (%s) is unknown", args.ReplyHash)
	}

input:

	// Ensure the reactions are all supported
	for _, reaction := range args.Reactions {
		if !util.IsEmojiValid(strings.TrimPrefix(reaction, "-")) {
			return fmt.Errorf("reaction (%s) is not supported", reaction)
		}
	}

	// When intent is to reply to a comment, a merge request number is required
	if args.MergeRequestNumber == 0 && args.ReplyHash != "" {
		return fmt.Errorf("merge request number is required when adding a comment")
	}

	// Hook to syscall.SIGINT signal so we close args.StdIn
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; args.StdIn.Close() }()

	// Prompt user for title only if was not provided via flag and this is not a comment
	if len(args.Title) == 0 && args.ReplyHash == "" && nComments == 0 {
		args.Title = args.InputReader("\033[1;33m? Title: (256 chars max.)\u001B[0m\n  ", &util.InputReaderArgs{})
		if len(args.Title) == 0 {
			return common.ErrTitleRequired
		}
	}

	// Read body from stdIn only if an editor is not requested and --no-body is unset
	if len(args.Body) == 0 && args.UseEditor == false && !args.NoBody {
		args.Body = args.InputReader("\033[1;33m? Body: (8192 chars max. | Tap enter thrice to exit)\033[0m\n  ",
			&util.InputReaderArgs{Multiline: true})
		fmt.Fprint(args.StdOut, "\010\010")
	}

	// Read body from editor if requested
	if args.UseEditor == true {
		var editor = args.EditorPath
		if editor == "" {
			editor = r.GetConfig("core.editor")
		}
		args.Body, err = args.EditorReader(editor, args.StdIn, os.Stdout, os.Stderr)
		if err != nil {
			return errors.Wrap(err, "failed read body from editor")
		}
	}

	// Body is required for a new merge request
	if nComments == 0 && args.Body == "" {
		return common.ErrBodyRequired
	}

	// Create the post body
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{
		Content:   []byte(args.Body),
		Title:     args.Title,
		ReplyTo:   args.ReplyHash,
		Reactions: args.Reactions,
		MergeRequestFields: common2.MergeRequestFields{
			BaseBranch:       args.Base,
			BaseBranchHash:   args.BaseHash,
			TargetBranch:     args.Target,
			TargetBranchHash: args.TargetHash,
		},
		Close: args.Close,
	})

	// Create a new merge request reference or add comment commit to the existing one
	newPost, ref, err := args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type:             plumbing.MergeRequestBranchPrefix,
		PostRefID:        args.MergeRequestNumber,
		Body:             postBody,
		IsComment:        args.ReplyHash != "",
		FreePostIDGetter: plumbing.GetFreePostID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create new merge request or new comment")
	}

	if newPost {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#0", ref))
	} else {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#%d", ref, nComments))
	}

	return nil
}
