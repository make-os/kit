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
func IssueCreateCmd(title, body, replyTo string, labels, assignees, fixers []string,
	useEditor bool,
	editorPath,
	targetIssue string,
	stdOut io.Writer,
	gitBinPath string) error {

	if targetIssue != "" && title != "" {
		return fmt.Errorf("title not required when adding a comment to an issue")
	}

	// Get the repository in the current working directory
	targetRepo, err := repo.GetRepoAtWorkingDir(gitBinPath)
	if err != nil {
		return errors.Wrap(err, "failed to open repo at cwd")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() { <-sigs; os.Stdin.Close() }()
	rdr := bufio.NewReader(os.Stdin)

	// Read title if not provided via flag and replyTo is not set
	if len(title) == 0 && len(replyTo) == 0 && len(targetIssue) == 0 {
		fmt.Fprintln(stdOut, color.HiBlackString("Title: (256 chars) - Press enter to continue"))
		title, _ = rdr.ReadString('\n')
		title = strings.TrimSpace(title)
	}

	// Read body
	if len(body) == 0 && useEditor == false {
		fmt.Fprintln(stdOut, color.HiBlackString("Body: (8192 chars) - Press ctrl-D to continue"))
		bz, _ := ioutil.ReadAll(rdr)
		body = strings.TrimSpace(string(bz))
	}
	if len(strings.TrimSpace(body)) == 0 {
		return fmt.Errorf("body is required")
	}

	// Read body from editor if required
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
	issueBody := issues.MakeIssueBody(title, body, replyTo, labels, assignees, fixers)

	// Create a new issue or add comment commit to existing issue
	newIssue, ref, err := issues.AddIssueOrCommentCommitCmd(targetRepo, targetIssue, issueBody)
	if err != nil {
		return err
	}

	if newIssue {
		fmt.Fprintln(stdOut, color.GreenString("New issue created @ %s", ref))
	} else {
		fmt.Fprintln(stdOut, color.GreenString("New issue comment created @ %s", ref))
	}

	return nil
}
