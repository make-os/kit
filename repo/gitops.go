package repo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrRefNotFound = fmt.Errorf("ref not found")

	ErrNoCommits = fmt.Errorf("no commits")
)

// GitOps provides convenience methods that utilize
// the git tool to access and modify a repository
type GitOps struct {
	gitBinPath string
	path       string
}

// NewGitOps creates an instance of GitOps.
// binPath: Git executable path
// path: The target repository path
func NewGitOps(gitBinPath, path string) *GitOps {
	return &GitOps{gitBinPath: gitBinPath, path: path}
}

// RefDelete executes `git update-ref -d <refname>` to delete a reference
func (g *GitOps) RefDelete(refname string) error {
	_, err := execGitCmd(g.gitBinPath, g.path, "update-ref", "-d", refname)
	if err != nil {
		return errors.Wrap(err, "reference delete failed")
	}
	return nil
}

// RefUpdate executes `git update-ref <refname> <commit hash>` to update/create a reference
func (g *GitOps) RefUpdate(refname, commitHash string) error {
	_, err := execGitCmd(g.gitBinPath, g.path, "update-ref", refname, commitHash)
	if err != nil {
		return errors.Wrap(err, "reference update failed")
	}
	return nil
}

// TagDelete executes `git tag -d <tagname>` to delete a tag
func (g *GitOps) TagDelete(tagname string) error {
	_, err := execGitCmd(g.gitBinPath, g.path, "tag", "-d", tagname)
	if err != nil {
		return errors.Wrap(err, "tag delete failed")
	}
	return nil
}

// RefGet returns the hash content of a reference.
// Returns ErrRefNotFound if ref does not exist
func (g *GitOps) RefGet(refname string) (string, error) {
	out, err := execGitCmd(g.gitBinPath, g.path, "rev-parse", "--verify", refname)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Needed a single revision") {
			return "", ErrRefNotFound
		}
		return "", errors.Wrap(err, "failed to get ref hash")
	}
	return strings.TrimSpace(string(out)), nil
}

// GetRecentCommit gets the hash of the recent commit
// Returns ErrNoCommits if no commits exist
func (g *GitOps) GetRecentCommit() (string, error) {

	// Get current branch
	curBranch, err := g.GetHEAD(true)
	if err != nil {
		return "", err
	}

	numCommits, err := g.NumCommits(curBranch)
	if err != nil {
		return "", err
	}

	if numCommits == 0 {
		return "", ErrNoCommits
	}

	out, err := execGitCmd(g.gitBinPath, g.path, "rev-parse", "HEAD")
	if err != nil {
		return "", errors.Wrap(err, "failed to get recent commit")
	}

	return strings.TrimSpace(string(out)), nil
}

// GetHEAD returns the reference stored in HEAD
// short: When set to true, the full reference name is returned
func (g *GitOps) GetHEAD(short bool) (string, error) {

	var args = []string{"symbolic-ref", "HEAD"}
	if short {
		args = []string{"symbolic-ref", "--short", "HEAD"}
	}

	out, err := execGitCmd(g.gitBinPath, g.path, args...)
	if err != nil {
		return "", errors.Wrap(err, "failed to get current branch")
	}

	return strings.TrimSpace(string(out)), nil
}

// NumCommits gets the number of commits in a branch
func (g *GitOps) NumCommits(branch string) (int, error) {
	out, err := execGitCmd(g.gitBinPath, g.path, "--no-pager", "log", "--oneline", branch,
		`--pretty="%h"`)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") {
			return 0, nil
		}
		return 0, errors.Wrap(err, "failed to get commit count")
	}
	shortHashes := strings.Fields(strings.TrimSpace(string(out)))
	return len(shortHashes), nil
}

// GetConfig finds and returns a config value
func (g *GitOps) GetConfig(path string) string {
	out, err := execGitCmd(g.gitBinPath, g.path, "config", path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CreateAndOrSignQuietCommit creates and optionally sign a quiet commit.
// msg: The commit message.
// signingKey: The optional signing key. If provided, the commit is signed
// env: Optional environment variables to pass to the command.
func (g *GitOps) CreateAndOrSignQuietCommit(msg, signingKey string, env ...string) error {
	args := []string{"commit", "--quiet", "--allow-empty", "--file", "-"}
	if signingKey != "" {
		args = append(args, "--gpg-sign="+signingKey)
	}
	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	cmd.Stdin = strings.NewReader(msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to commit")
}

// CreateTagWithMsg an annotated tag
// args: `git tag` options (NOTE: -a and --file=- are added by default)
// msg: The tag's message which is passed to the command's stdin.
// signingKey: The signing key to use
// env: Optional environment variables to pass to the command.
func (g *GitOps) CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error {
	if signingKey != "" {
		args = append(args, "-u", signingKey)
	}
	args = append([]string{"tag", "-a", "--file", "-"}, args...)
	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	cmd.Stdin = strings.NewReader(msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to create tag")
}

// ListTreeObjects returns a map containing tree entries (filename: objectname)
func (g *GitOps) ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string, error) {
	args := []string{"ls-tree", treename}
	if recursive {
		args = append(args, "-r")
	}

	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Run()
	if err != nil {
		out.WriteTo(os.Stdout)
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var treeMap = make(map[string]string)
	for _, entry := range lines {
		fields := strings.Fields(entry)
		treeMap[fields[2]] = fields[3]
	}

	return treeMap, nil
}

// ListTreeObjectsSlice returns a slice containing objects name of tree entries
func (g *GitOps) ListTreeObjectsSlice(treename string, recursive, showTrees bool, env ...string) ([]string, error) {
	args := []string{"ls-tree", treename}
	if recursive {
		args = append(args, "-r")
	}
	if recursive && showTrees {
		args = append(args, "-t")
	}

	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	err := cmd.Run()
	if err != nil {
		out.WriteTo(os.Stdout)
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var entriesHash []string
	for _, entry := range lines {
		fields := strings.Fields(entry)
		entriesHash = append(entriesHash, fields[2])
	}

	return entriesHash, nil
}

// RemoveEntryFromNote removes a note
func (g *GitOps) RemoveEntryFromNote(notename, objectHash string, env ...string) error {
	args := []string{"notes", "--ref", notename, "add", "-m", "", "-f", objectHash}
	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to remove note")
}

// AddEntryToNote adds a note
func (g *GitOps) AddEntryToNote(notename, objectHash, note string, env ...string) error {
	args := []string{"notes", "--ref", notename, "add", "-m", note, "-f", objectHash}
	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to create note entry")
}

// CreateBlob creates a blob object
func (g *GitOps) CreateBlob(content string) (string, error) {
	cmd := exec.Command(g.gitBinPath, []string{"hash-object", "-w", "--stdin"}...)
	cmd.Dir = g.path
	cmd.Stdin = strings.NewReader(content)
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		out.WriteTo(os.Stdout)
		return "", errors.Wrap(err, "failed to create blob")
	}
	return strings.TrimSpace(out.String()), nil
}

// UpdateRecentCommitMsg updates the recent commit message
// msg: The commit message which is passed to the command's stdin.
// signingKey: The signing key
// env: Optional environment variables to pass to the command.
func (g *GitOps) UpdateRecentCommitMsg(msg, signingKey string, env ...string) error {
	args := []string{"commit", "--amend", "--quiet", "--allow-empty-message",
		"--allow-empty", "--file", "-"}
	if signingKey != "" {
		args = append(args, "--gpg-sign="+signingKey)
	}
	cmd := exec.Command(g.gitBinPath, args...)
	cmd.Dir = g.path
	cmd.Stdin = strings.NewReader(msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to update recent commit msg")
}
