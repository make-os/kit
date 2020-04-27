package cmd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
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

// readFromEditor reads input from the specified editor
func readFromEditor(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		return "", nil
	}
	defer os.Remove(file.Name())

	args := strings.Split(editor, " ")
	cmd := exec.Command(args[0], append(args[1:], file.Name())...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = stdIn
	if err := cmd.Run(); err != nil {
		return "", err
	}

	bz, err := ioutil.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	return string(bz), nil
}

type IssueCreateArg struct {
	// IssueID is the unique Issue ID
	IssueID int
	// Title is the title of the Issue
	Title string
	// Body is the Issue's body
	Body string
	// ReplyHash is the hash of a comment commit
	ReplyHash string
	// Reactions towards the replied comment commit
	Reactions []string
	// Labels may include terms used to classify the Issue
	Labels []string
	// Assignees may include push keys that may be interpreted by an application
	Assignees []string
	// Fixers may include push keys of indicating who should fix the Issue
	Fixers []string
	// UseEditor indicates that the body of the Issue should be collected using a text editor.
	UseEditor bool
	// EditorPath indicates the path to an editor program
	EditorPath string
	// NoBody prevents prompting user for issue body
	NoBody bool
	// StdOut receives the output
	StdOut io.Writer
	// StdIn receives input
	StdIn io.ReadCloser
}

// IssueCreateCmd create a new Issue or adds a comment commit to an existing Issue
func IssueCreateCmd(targetRepo core.BareRepo, args IssueCreateArg) error {

	var nIssueComments int
	var issueRef, issueRefHash string
	var err error

	// If issue number is not set, it means a new issue should be created.
	// Go directly to input collection.
	if args.IssueID == 0 {
		goto input
	}

	// Get the issue reference
	issueRef = plumbing.MakeIssueReference(args.IssueID)
	issueRefHash, err = targetRepo.RefGet(issueRef)

	// When issue does not exist and this is a reply intent, return error
	if err != nil && err == repo.ErrRefNotFound && args.ReplyHash != "" {
		return fmt.Errorf("issue (%d) was not found", args.IssueID)
	} else if err != nil && err != repo.ErrRefNotFound {
		return err
	}

	// Get the number of comments
	nIssueComments, err = targetRepo.NumCommits(issueRef, false)
	if err != nil {
		return fmt.Errorf("failed to count comments in issue")
	}

	// Title is not required when the intent is to add a comment
	if nIssueComments > 0 && args.Title != "" {
		return fmt.Errorf("title not required when adding a comment to an issue")
	}

	// When the intent is to reply to a specific comment, ensure the target
	// comment hash exist in the issue
	if args.ReplyHash != "" {
		if targetRepo.IsAncestor(args.ReplyHash, issueRefHash) != nil {
			return fmt.Errorf("target comment hash (%s) is unknown", args.ReplyHash)
		}
	}

input:

	// When reactions are set, ensure the intent is to add a comment
	if len(args.Reactions) > 0 && args.ReplyHash == "" && nIssueComments == 0 {
		return fmt.Errorf("reactions can only be used when adding a comment")
	}

	// Ensure the reactions are all supported
	for _, reaction := range args.Reactions {
		if _, ok := util.EmojiCodeMap[reaction]; !ok {
			return fmt.Errorf("reaction (%s) is not supported", reaction)
		}
	}

	// When intent is to reply to a comment, an issue number is required
	if args.IssueID == 0 && args.ReplyHash != "" {
		return fmt.Errorf("issue number is required")
	}

	// Hook to syscall.SIGINT signal so we close args.StdIn
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; args.StdIn.Close() }()
	rdr := bufio.NewReader(args.StdIn)

	// Prompt user for title only if was not provided via flag and this is not a comment
	if len(args.Title) == 0 && args.ReplyHash == "" && nIssueComments == 0 {
		fmt.Fprintln(args.StdOut, color.HiBlackString("Title: (256 chars) - Press enter to continue"))
		args.Title, _ = rdr.ReadString('\n')
		args.Title = strings.TrimSpace(args.Title)
	}

	// Read body from stdIn only if an editor is not requested and --no-body is unset
	if len(args.Body) == 0 && args.UseEditor == false && !args.NoBody {
		fmt.Fprintln(args.StdOut, color.HiBlackString("Body: (8192 chars) - Press ctrl-D to continue"))
		bz, _ := ioutil.ReadAll(rdr)
		fmt.Fprint(args.StdOut, "\n")
		args.Body = strings.TrimSpace(string(bz))
	}

	// Ensure body is provided. But not required when a reaction is provided
	if len(strings.TrimSpace(args.Body)) == 0 && len(args.Reactions) == 0 {
		return fmt.Errorf("body is required")
	}

	// Read body from editor if requested
	if args.UseEditor == true {
		editor := targetRepo.GetConfig("core.editor")
		if args.EditorPath != "" {
			editor = args.EditorPath
		}
		args.Body, err = readFromEditor(editor, args.StdIn, os.Stdout, os.Stderr)
		if err != nil {
			return errors.Wrap(err, "failed read body from editor")
		}
	}

	// Create the Issue body and prompt user to confirm
	issueBody := issues.MakeIssueBody(args.Title, args.Body, args.ReplyHash,
		args.Reactions, args.Labels, args.Assignees, args.Fixers)

	// Create a new Issue or add comment commit to existing Issue
	newIssue, ref, err := issues.AddIssueOrCommentCommit(targetRepo, args.IssueID, issueBody, args.ReplyHash != "")
	if err != nil {
		return err
	}

	if newIssue {
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#0", ref))
	} else {
		id, _ := targetRepo.NumCommits(ref, false)
		fmt.Fprintln(args.StdOut, fmt.Sprintf("%s#%d", ref, id-1))
	}

	return nil
}
