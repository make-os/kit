package repo

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// GitOps errors
var (
	ErrRefNotFound = fmt.Errorf("ref not found")
	ErrEmptyHEAD   = fmt.Errorf("empty head")
	ErrNoCommits   = fmt.Errorf("no commits")
)

// GitOps provides git operations
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

// UpdateRecentCommitMsg updates the recent commit message
func (g *GitOps) UpdateRecentCommitMsg(msg string) error {
	cmd := exec.Command(g.gitBinPath, "commit", "--amend",
		"--quiet", "--allow-empty-message", "--file", "-")
	cmd.Dir = g.path
	cmd.Stdin = strings.NewReader(msg)
	return errors.Wrap(cmd.Run(), "failed to update recent commit msg")
}
