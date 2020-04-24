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

// IssueCreateCmd create a new issue or adds a comment commit to an existing issue
func IssueCreateCmd(
	title,
	body string,
	replyHash string,
	labels,
	assignees,
	fixers []string,
	useEditor bool,
	editorPath string,
	stdOut io.Writer,
	gitBinPath string,
	issueID int) error {

	// When an issue ID is not set and a target comment commit is set,
	// it means this the intent is to add reply to a comment.
	// Ensure the issue ID where the comment commit reside is set.
	if issueID == 0 && replyHash != "" {
		return fmt.Errorf("issue ID is required")
	}

	// Get the repository in the current working directory where the command is being run
	targetRepo, err := repo.GetRepoAtWorkingDir(gitBinPath)
	if err != nil {
		return errors.Wrap(err, "failed to open repo at cwd")
	}

	var nIssueComments int
	if issueID > 0 {

		// Get the number of comment commits in the issue
		issueRef := plumbing.MakeIssueReference(issueID)
		nIssueComments, err = targetRepo.NumCommits(issueRef, false)
		if err != nil {
			return fmt.Errorf("failed to count comment commits in issue")
		}

		// Do not allow a title when this action will result in a
		// comment on the issue. Comments do not carry titles.
		if nIssueComments > 0 && title != "" {
			return fmt.Errorf("title not required when adding a comment to an issue")
		}

		// When a reply ID is set, this is a reply to another comment commit
		// using their commit hash. Ensure the hash exist in the history of the issue
		if replyHash != "" {
			issueRefHash, _ := targetRepo.RefGet(issueRef)
			if targetRepo.IsAncestor(replyHash, issueRefHash) != nil {
				return fmt.Errorf("target comment hash (%s) is unknown", replyHash)
			}
		}
	}

	// Hook to syscall.SIGINT signal so we close os.Stdin
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; os.Stdin.Close() }()
	rdr := bufio.NewReader(os.Stdin)

	// Prompt user for title only if was not provided via flag and this is not a comment
	if len(title) == 0 && replyHash == "" && nIssueComments == 0 {
		fmt.Fprintln(stdOut, color.HiBlackString("Title: (256 chars) - Press enter to continue"))
		title, _ = rdr.ReadString('\n')
		title = strings.TrimSpace(title)
	}

	// Read body from stdIn only if an editor is not requested
	if len(body) == 0 && useEditor == false {
		fmt.Fprintln(stdOut, color.HiBlackString("Body: (8192 chars) - Press ctrl-D to continue"))
		bz, _ := ioutil.ReadAll(rdr)
		body = strings.TrimSpace(string(bz))
	}

	// No matter what, body is always required
	if len(strings.TrimSpace(body)) == 0 {
		return fmt.Errorf("body is required")
	}

	// Read body from editor if requested
	if useEditor == true {
		editor := targetRepo.GetConfig("core.editor")
		if editorPath != "" {
			editor = editorPath
		}
		body, err = readFromEditor(editor, os.Stdin, os.Stdout, os.Stderr)
		if err != nil {
			return errors.Wrap(err, "failed read body from editor")
		}
	}

	// Create the issue body and prompt user to confirm
	issueBody := issues.MakeIssueBody(title, body, replyHash, labels, assignees, fixers)

	// Create a new issue or add comment commit to existing issue
	newIssue, ref, err := issues.AddIssueOrCommentCommit(targetRepo, issueID, issueBody, replyHash != "")
	if err != nil {
		return err
	}

	if newIssue {
		fmt.Fprintln(stdOut, fmt.Sprintf("%s#0", ref))
	} else {
		id, _ := targetRepo.NumCommits(ref, false)
		fmt.Fprintln(stdOut, fmt.Sprintf("%s#%d", ref, id-1))
	}

	return nil
}
