package repo

import "github.com/pkg/errors"

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

// HardReset executes `git reset --hard <commit>` to reset
// the repository to the given commit
func (g *GitOps) HardReset(commit string) error {
	_, err := execGitCmd(g.gitBinPath, g.path, "reset", "--hard", commit)
	if err != nil {
		return errors.Wrap(err, "hard reset failed")
	}
	return nil
}

// RefDelete executes `git update-ref -d <refname>` to delete a reference
func (g *GitOps) RefDelete(refname string) error {
	_, err := execGitCmd(g.gitBinPath, g.path, "update-ref", "-d", refname)
	if err != nil {
		return errors.Wrap(err, "reference delete failed")
	}
	return nil
}

// RefUpdate executes `git update-ref <refname> <commit hash>` to delete a reference
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
