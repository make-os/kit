package issuecmd

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

type IssueCreateArgs struct {
	// IssueID is the unique Issue ID
	ID int

	// Title is the title of the Issue
	Title string

	// Body is the Issue's body
	Body string

	// ReplyHash is the hash of a comment commit
	ReplyHash string

	// Reactions adds or removes reactions to/from a comment commit
	// Negated reactions indicate removal request
	Reactions []string

	// Labels may include terms used to classify the Issue
	Labels *[]string

	// Assignees may include push keys that may be interpreted by an application
	Assignees *[]string

	// UseEditor indicates that the body of the Issue should be collected using a text editor.
	UseEditor bool

	// EditorPath indicates the path to an editor program
	EditorPath string

	// NoBody prevents prompting user for issue body
	NoBody bool

	// Close sets close status to 1.
	Close *bool

	// Open sets close status to 0
	Open *bool

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
	InputReader util.InputReader
}

// IssueCreateCmd create a new Issue or adds a comment commit to an existing Issue
func IssueCreateCmd(r types.LocalRepo, args *IssueCreateArgs) error {

	var nComments int
	var issueRef, issueRefHash string
	var err error

	if args.ID != 0 {

		// Get the issue reference
		issueRef = plumbing.MakeIssueReference(args.ID)
		issueRefHash, err = r.RefGet(issueRef)

		// When issue does not exist and this is a reply intent, return error
		if err != nil && err == plumbing.ErrRefNotFound && args.ReplyHash != "" {
			return fmt.Errorf("issue (%d) was not found", args.ID)
		} else if err != nil && err != plumbing.ErrRefNotFound {
			return err
		}

		// Get the number of comments
		nComments, err = r.NumCommits(issueRef, false)
		if err != nil {
			return errors.Wrap(err, "failed to count comments in issue")
		}

		// Title is not required when the intent is to add a comment
		if nComments > 0 && args.Title != "" {
			return fmt.Errorf("title not required when adding a comment to an issue")
		}

		// When the intent is to reply to a specific comment, ensure the target
		// comment hash exist in the issue
		if args.ReplyHash != "" && r.IsAncestor(args.ReplyHash, issueRefHash) != nil {
			return fmt.Errorf("target comment hash (%s) is unknown", args.ReplyHash)
		}
	}

	// Ensure the reactions are all supported
	for _, reaction := range args.Reactions {
		if !util.IsEmojiValid(strings.TrimPrefix(reaction, "-")) {
			return fmt.Errorf("reaction (%s) is not supported", reaction)
		}
	}

	// Ensure labels are valid identifiers
	if args.Labels != nil {
		for _, label := range *args.Labels {
			if err := util.IsValidNameNoLen(strings.TrimPrefix(label, "-")); err != nil {
				return fmt.Errorf("label (%s) is not valid", label)
			}
		}
	}

	// Ensure assignees are valid push address
	if args.Assignees != nil {
		for _, assignee := range *args.Assignees {
			if !util.IsValidPushAddr(strings.TrimPrefix(assignee, "-")) {
				return fmt.Errorf("assignee (%s) is not a valid push key address", assignee)
			}
		}
	}

	// When intent is to reply to a comment, an issue number is required
	if args.ID == 0 && args.ReplyHash != "" {
		return fmt.Errorf("issue number is required when adding a comment")
	}

	// Hook to syscall.SIGINT signal to close args.StdIn on interrupt
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; args.StdIn.Close() }()

	// Prompt user for title only if was not provided via flag and this is not a comment
	if len(args.Title) == 0 && args.ReplyHash == "" && nComments == 0 {
		args.Title = args.InputReader("\033[1;32m? \033[1;37mTitle> \u001B[0m", &util.InputReaderArgs{
			After: func(input string) { fmt.Fprintf(args.StdOut, "\033[36m%s\033[0m\n", input) },
		})
		if len(args.Title) == 0 {
			return common.ErrTitleRequired
		}
	}

	// Read body from stdIn only if an editor is not requested and --no-body is unset
	if len(args.Body) == 0 && args.UseEditor == false && !args.NoBody {
		args.Body = args.InputReader("\033[1;32m? \033[1;37mBody> \u001B[0m", &util.InputReaderArgs{
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

	// Body is required for a new issue
	if nComments == 0 && args.Body == "" {
		return common.ErrBodyRequired
	}

	// Create the post body
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{
		Content:   []byte(args.Body),
		Title:     args.Title,
		ReplyTo:   args.ReplyHash,
		Reactions: args.Reactions,
		IssueFields: common2.IssueFields{
			Labels:    args.Labels,
			Assignees: args.Assignees,
		},
		Close: args.Close,
	})

	// Create a new Issue or add comment commit to existing Issue
	newIssue, ref, err := args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type:          plumbing.IssueBranchPrefix,
		ID:            args.ID,
		Body:          postBody,
		IsComment:     args.ReplyHash != "",
		Force:         args.Force,
		GetFreePostID: plumbing.GetFreePostID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create or add new comment to issue")
	}

	if newIssue {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#0", ref))
	} else {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#%d", ref, nComments))
	}

	return nil
}
