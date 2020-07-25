package mergecmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/util"
	fmt2 "github.com/themakeos/lobe/util/colorfmt"
	io2 "github.com/themakeos/lobe/util/io"
)

type MergeRequestCreateArgs struct {
	// ID is the unique post ID
	ID int

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

	// Force indicates that uncommitted changes should be ignored
	Force bool

	// StdOut receives the output
	StdOut io.Writer

	// StdIn receives input
	StdIn io.ReadCloser

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// EditorReader is used to read from an editor program
	EditorReader util.EditorReaderFunc

	// InputReader is a function that reads input from stdin
	InputReader io2.InputReader
}

// MergeRequestCreateCmd create a new merge request or adds a comment to an existing one
func MergeRequestCreateCmd(r types.LocalRepo, args *MergeRequestCreateArgs) error {

	var nComments int
	var mrRef, mrRefHash string
	var err error

	if args.ID != 0 {

		// Get the merge request reference name and attempt to get it
		mrRef = plumbing.MakeMergeRequestReference(args.ID)
		mrRefHash, err = r.RefGet(mrRef)

		// When the merge request does not exist and this is a reply intent, return error
		if err != nil && err == plumbing.ErrRefNotFound && args.ReplyHash != "" {
			return fmt.Errorf("merge request (%d) was not found", args.ID)
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

		// Base and target information are required for new merge request
		if nComments == 0 {
			if args.Base == "" {
				return fmt.Errorf("base branch name is required")
			}
			if args.BaseHash == "" {
				return fmt.Errorf("base branch hash is required")
			}
			if args.Target == "" {
				return fmt.Errorf("target branch name is required")
			}
			if args.TargetHash == "" {
				return fmt.Errorf("target branch hash is required")
			}
		}
	}

	// Ensure the reactions are all supported
	for _, reaction := range args.Reactions {
		if !util.IsEmojiValid(strings.TrimPrefix(reaction, "-")) {
			return fmt.Errorf("reaction (%s) is not supported", reaction)
		}
	}

	// When intent is to reply to a comment, a merge request number is required
	if args.ID == 0 && args.ReplyHash != "" {
		return fmt.Errorf("merge request number is required when adding a comment")
	}

	// Hook to syscall.SIGINT signal so we close args.StdIn
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; args.StdIn.Close() }()

	// Prompt user for title only if was not provided via flag and this is not a comment
	if len(args.Title) == 0 && args.ReplyHash == "" && nComments == 0 {
		args.Title = args.InputReader("\033[1;32m? \033[1;37mTitle> \u001B[0m", &io2.InputReaderArgs{
			After: func(input string) { fmt.Fprintf(args.StdOut, "\033[36m%s\033[0m\n", input) },
		})
		if len(args.Title) == 0 {
			return common.ErrTitleRequired
		}
	}

	// Read body from stdIn only if an editor is not requested and --no-body is unset
	if len(args.Body) == 0 && args.UseEditor == false && !args.NoBody {
		args.Body = args.InputReader("\033[1;32m? \033[1;37mBody> \u001B[0m", &io2.InputReaderArgs{
			After: func(input string) { fmt.Fprintf(args.StdOut, "\033[36m%s\033[0m\n", input) },
		})
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
		if args.Body != "" {
			fmt.Fprint(args.StdOut, "\u001B[1;32m? \u001B[1;37mBody> \033[0m\u001B[36m[Received]\u001B[0m\n")
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
		MergeRequestFields: types.MergeRequestFields{
			BaseBranch:       args.Base,
			BaseBranchHash:   args.BaseHash,
			TargetBranch:     args.Target,
			TargetBranchHash: args.TargetHash,
		},
		Close: args.Close,
	})

	// Create a new merge request reference or add comment commit to the existing one
	newPost, ref, err := args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type:          plumbing.MergeRequestBranchPrefix,
		ID:            args.ID,
		Body:          postBody,
		IsComment:     args.ReplyHash != "",
		Force:         args.Force,
		GetFreePostID: plumbing.GetFreePostID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create or add new comment to merge request request")
	}

	if newPost {
		fmt.Fprintln(args.StdOut, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ New merge request created!"))
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#0", ref))
	} else {
		fmt.Fprintln(args.StdOut, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ New comment added!"))
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#%d", ref, nComments))
	}

	return nil
}
