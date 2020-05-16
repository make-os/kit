package issuecmd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/issues"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

type IssueCreateArgs struct {
	// IssueID is the unique Issue ID
	IssueNumber int

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
	Labels []string

	// Assignees may include push keys that may be interpreted by an application
	Assignees []string

	// UseEditor indicates that the body of the Issue should be collected using a text editor.
	UseEditor bool

	// EditorPath indicates the path to an editor program
	EditorPath string

	// NoBody prevents prompting user for issue body
	NoBody bool

	// Close sets close status to 1.
	Close *bool

	// Open sets close status to 0
	Open bool

	// StdOut receives the output
	StdOut io.Writer

	// StdIn receives input
	StdIn io.ReadCloser

	// IssueOrCommentCommitCreatorFunc is the issue commit creating function
	IssueCommentCreator issues.IssueCommentCreator

	// EditorReader is used to read from an editor program
	EditorReader func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error)
}

var (
	ErrBodyRequired  = fmt.Errorf("body is required")
	ErrTitleRequired = fmt.Errorf("title is required")
)

// IssueCreateCmd create a new Issue or adds a comment commit to an existing Issue
func IssueCreateCmd(r core.LocalRepo, args *IssueCreateArgs) error {

	var nComments int
	var issueRef, issueRefHash string
	var err error

	// If issue number is not set, it means a new issue should be created.
	// Go directly to input collection.
	if args.IssueNumber == 0 {
		goto input
	}

	// Get the issue reference
	issueRef = plumbing.MakeIssueReference(args.IssueNumber)
	issueRefHash, err = r.RefGet(issueRef)

	// When issue does not exist and this is a reply intent, return error
	if err != nil && err == repo.ErrRefNotFound && args.ReplyHash != "" {
		return fmt.Errorf("issue (%d) was not found", args.IssueNumber)
	} else if err != nil && err != repo.ErrRefNotFound {
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

input:

	// Ensure the reactions are all supported
	for _, reaction := range args.Reactions {
		if !util.IsEmojiValid(strings.TrimPrefix(reaction, "-")) {
			return fmt.Errorf("reaction (%s) is not supported", reaction)
		}
	}

	// Ensure labels are valid identifiers
	for _, label := range args.Labels {
		if err := util.IsValidNameNoLen(strings.TrimPrefix(label, "-")); err != nil {
			return fmt.Errorf("label (%s) is not valid", label)
		}
	}

	// Ensure assignees are valid push address
	for _, assignee := range args.Assignees {
		if !util.IsValidPushAddr(strings.TrimPrefix(assignee, "-")) {
			return fmt.Errorf("assignee (%s) is not a valid push key address", assignee)
		}
	}

	// When intent is to reply to a comment, an issue number is required
	if args.IssueNumber == 0 && args.ReplyHash != "" {
		return fmt.Errorf("issue number is required when adding a comment")
	}

	// Hook to syscall.SIGINT signal so we close args.StdIn
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; args.StdIn.Close() }()
	rdr := bufio.NewReader(args.StdIn)

	// Prompt user for title only if was not provided via flag and this is not a comment
	if len(args.Title) == 0 && args.ReplyHash == "" && nComments == 0 {
		fmt.Fprintln(args.StdOut, color.HiBlackString("Title: (256 chars) - Press enter to continue"))
		args.Title, _ = rdr.ReadString('\n')
		args.Title = strings.TrimSpace(args.Title)
		if len(args.Title) == 0 {
			return ErrTitleRequired
		}
	}

	// Read body from stdIn only if an editor is not requested and --no-body is unset
	if len(args.Body) == 0 && args.UseEditor == false && !args.NoBody {
		fmt.Fprintln(args.StdOut, color.HiBlackString("Body: (8192 chars) - Press ctrl-D to continue"))
		bz, _ := ioutil.ReadAll(rdr)
		fmt.Fprint(args.StdOut, "\n")
		args.Body = strings.TrimSpace(string(bz))
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

	// Body is required for a new issue
	if nComments == 0 && args.Body == "" {
		return ErrBodyRequired
	}

	// Create the Issue body and prompt user to confirm
	issueBody := plumbing.IssueBodyToString(&plumbing.IssueBody{
		Content:   []byte(args.Body),
		Title:     args.Title,
		ReplyTo:   args.ReplyHash,
		Reactions: args.Reactions,
		Labels:    &args.Labels,
		Assignees: &args.Assignees,
		Close:     args.Close,
	})

	// Create a new Issue or add comment commit to existing Issue
	newIssue, ref, err := args.IssueCommentCreator(r, args.IssueNumber, issueBody, args.ReplyHash != "")
	if err != nil {
		return errors.Wrap(err, "failed to add issue or comment")
	}

	if newIssue {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#0", ref))
	} else {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#%d", ref, nComments))
	}

	return nil
}
