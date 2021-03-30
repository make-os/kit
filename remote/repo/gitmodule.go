package repo

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/make-os/kit/remote/plumbing"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gopkg.in/src-d/go-git.v4"
)

type InitRepositoryFunc func(name string, rootDir string, gitBinPath string) error

// execGitCmd executes git commands and returns the output
// repoDir: The directory of the target repository.
// args: Arguments for the git sub-command
func ExecGitCmd(gitBinDir, repoDir string, args ...string) ([]byte, error) {
	cmd := exec.Command(gitBinDir, args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, errors.Wrap(err, fmt.Sprintf("exec error: cmd=%s, output=%s",
			cmd.String(), string(out)))
	}
	return out, nil
}

// InitRepository creates a bare git repository
func InitRepository(name, rootDir, gitBinPath string) error {

	// Create the repository
	path := filepath.Join(rootDir, name)
	_, err := git.PlainInit(path, true)
	if err != nil {
		return errors.Wrap(err, "failed to create repo")
	}

	// Set config options
	options := [][]string{
		{"gc.auto", "0"},
	}
	for _, opt := range options {
		_, err = ExecGitCmd(gitBinPath, path, append([]string{"config"}, opt...)...)
		if err != nil {
			return errors.Wrap(err, "failed to set config")
		}
	}

	return err
}

// GitModule provides convenience methods that utilize
// the git tool to access and modify a repository
type GitModule struct {
	gitBinPath string
	path       string
}

// NewGitModule creates an instance of GitModule.
// binPath: Git executable path
// path: The target repository path
func NewGitModule(gitBinPath, path string) *GitModule {
	return &GitModule{gitBinPath: gitBinPath, path: path}
}

// RefDelete executes `git update-ref -d <refname>` to delete a reference
func (gm *GitModule) RefDelete(refname string) error {
	_, err := ExecGitCmd(gm.gitBinPath, gm.path, "update-ref", "-d", refname)
	if err != nil {
		return errors.Wrap(err, "reference delete failed")
	}
	return nil
}

// RefUpdate executes `git update-ref <refname> <commit hash>` to update/create a reference
func (gm *GitModule) RefUpdate(refname, commitHash string) error {
	_, err := ExecGitCmd(gm.gitBinPath, gm.path, "update-ref", refname, commitHash)
	if err != nil {
		return errors.Wrap(err, "reference update failed")
	}
	return nil
}

// TagDelete executes `git tag -d <tagname>` to delete a tag
func (gm *GitModule) TagDelete(tagname string) error {
	_, err := ExecGitCmd(gm.gitBinPath, gm.path, "tag", "-d", tagname)
	if err != nil {
		return errors.Wrap(err, "tag delete failed")
	}
	return nil
}

// RefGet returns the hash content of a reference.
// Returns ErrRefNotFound if ref does not exist
func (gm *GitModule) RefGet(refname string) (string, error) {
	out, err := ExecGitCmd(gm.gitBinPath, gm.path, "rev-parse", "--verify", refname)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Needed a single revision") {
			return "", plumbing.ErrRefNotFound
		}
		return "", errors.Wrap(err, "failed to get ref hash")
	}
	return strings.TrimSpace(string(out)), nil
}

// GetRecentCommitHash gets the hash of the recent commit
// Returns ErrNoCommits if no commits exist
func (gm *GitModule) GetRecentCommitHash() (string, error) {

	// Get current branch
	curBranch, err := gm.GetHEAD(true)
	if err != nil {
		return "", err
	}

	numCommits, err := gm.NumCommits(curBranch, false)
	if err != nil {
		return "", err
	}

	if numCommits == 0 {
		return "", plumbing.ErrNoCommits
	}

	out, err := ExecGitCmd(gm.gitBinPath, gm.path, "rev-parse", "HEAD")
	if err != nil {
		return "", errors.Wrap(err, "failed to get recent commit")
	}

	return strings.TrimSpace(string(out)), nil
}

// GetHEAD returns the reference stored in HEAD
// short: When set to true, the full reference name is returned
func (gm *GitModule) GetHEAD(short bool) (string, error) {

	var args = []string{"symbolic-ref", "HEAD"}
	if short {
		args = []string{"symbolic-ref", "--short", "HEAD"}
	}

	out, err := ExecGitCmd(gm.gitBinPath, gm.path, args...)
	if err != nil {
		return "", errors.Wrap(err, "failed to get current branch")
	}

	return strings.TrimSpace(string(out)), nil
}

// CreateEmptyCommit creates a quiet commit.
// msg: The commit message.
// signingKey: The optional signing key. If provided, the commit is signed
// env: Optional environment variables to pass to the command.
func (gm *GitModule) CreateEmptyCommit(msg, signingKey string, env ...string) error {
	args := []string{"commit", "--quiet", "--allow-empty", "--allow-empty-message", "--file", "-"}
	if signingKey != "" {
		args = append(args, "--gpg-sign="+signingKey)
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
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
func (gm *GitModule) CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error {
	if signingKey != "" {
		args = append(args, "-u", signingKey)
	}
	args = append([]string{"tag", "-a", "--file", "-"}, args...)
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Stdin = strings.NewReader(msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to create tag")
}

// ListTreeObjects returns a map containing tree entries (filename: objectname)
func (gm *GitModule) ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string, error) {
	args := []string{"ls-tree", treename}
	if recursive {
		args = append(args, "-r")
	}

	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
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
func (gm *GitModule) ListTreeObjectsSlice(treename string, recursive, showTrees bool, env ...string) ([]string, error) {
	args := []string{"ls-tree", treename}
	if recursive {
		args = append(args, "-r")
	}
	if recursive && showTrees {
		args = append(args, "-t")
	}

	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
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
func (gm *GitModule) RemoveEntryFromNote(notename, objectHash string, env ...string) error {
	args := []string{"notes", "--ref", notename, "add", "-m", "", "-f", objectHash}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to remove note")
}

// AddEntryToNote adds a note
func (gm *GitModule) AddEntryToNote(notename, objectHash, note string, env ...string) error {
	args := []string{"notes", "--ref", notename, "add", "-m", note, "-f", objectHash}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to create note entry")
}

// CreateBlob creates a blob object
func (gm *GitModule) CreateBlob(content string) (string, error) {
	cmd := exec.Command(gm.gitBinPath, []string{"hash-object", "-w", "--stdin"}...)
	cmd.Dir = gm.path
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

// AmendRecentCommitWithMsg amends the recent commit
// msg: The commit message.
// signingKey: An optional signing key
// env: Optional environment variables to pass to the command.
func (gm *GitModule) AmendRecentCommitWithMsg(msg, signingKey string, env ...string) error {
	args := []string{"commit", "--amend", "--quiet", "--allow-empty-message",
		"--allow-empty", "--file", "-"}
	if signingKey != "" {
		args = append(args, "--gpg-sign="+signingKey)
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Stdin = strings.NewReader(msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return errors.Wrap(cmd.Run(), "failed to update recent commit msg")
}

// GetMergeCommits returns the hash of merge commits in a reference
func (gm *GitModule) GetMergeCommits(reference string, env ...string) ([]string, error) {
	args := []string{"--no-pager", "log", "--merges", "--oneline", "--format=%H", reference}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)

	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(cmd.Run(), "failed to get merges")
	}

	if trimmed := strings.TrimSpace(out.String()); trimmed != "" {
		hashes := strings.Split(trimmed, ",")
		return hashes, nil
	}

	return []string{}, nil
}

// HasMergeCommits checks whether a reference has merge commits
func (gm *GitModule) HasMergeCommits(reference string, env ...string) (bool, error) {
	hashes, err := gm.GetMergeCommits(reference, env...)
	if err != nil {
		return false, err
	}
	return len(hashes) > 0, nil
}

// CreateSingleFileCommit creates a commit tree with no parent and has only one file
func (gm *GitModule) CreateSingleFileCommit(filename, content, commitMsg, parent string) (string, error) {

	// Create body blob
	args := []string{"hash-object", "-w", "--stdin"}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	blobHash := strings.TrimSpace(string(out))

	// Create the tree hash
	args = []string{"mktree"}
	cmd = exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Stdin = strings.NewReader(fmt.Sprintf("100644 blob %s\t%s", blobHash, filename))
	out, err = cmd.Output()
	if err != nil {
		return "", err
	}
	treeHash := strings.TrimSpace(string(out))

	// Create the commit tree
	args = []string{"commit-tree", treeHash}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	if commitMsg != "" {
		args = append(args, "-m", commitMsg)
	}
	cmd = exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out, err = cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// NumCommits counts the number of commits in a reference.
// When noMerges is true, merges are not counted.
func (gm *GitModule) NumCommits(refname string, noMerges bool) (int, error) {
	args := []string{"rev-list", "--count", refname}
	if noMerges {
		args = append(args, "--no-merges")
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		if strings.Contains(out.String(), "unknown revision") {
			return 0, nil
		}
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(out.String()))
}

// Checkout switches HEAD to the specified reference.
// When create is true, the -b is added
func (gm *GitModule) Checkout(refname string, create, force bool) error {
	args := []string{"checkout", "--quiet"}
	if create {
		args = append(args, "-b", refname)
	} else {
		args = append(args, refname)
	}
	if force {
		args = append(args, "-f")
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		outStr := out.String()
		if strings.Contains(outStr, "did not match any file(s) known to git") {
			return plumbing.ErrRefNotFound
		}
		return errors.Wrap(err, outStr)
	}
	return nil
}

// GetRefCommits returns the hash of all commits in the specified reference's history
func (gm *GitModule) GetRefCommits(ref string, noMerges bool) ([]string, error) {
	args := []string{"rev-list", ref}
	if noMerges {
		args = append(args, "--no-merges")
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		outStr := out.String()
		if strings.Contains(outStr, "unknown revision or path") {
			return nil, plumbing.ErrRefNotFound
		}
		return nil, errors.Wrap(err, outStr)
	}

	return strings.Fields(out.String()), nil
}

// GetRefRootCommit returns the hash of the root commit of the specified reference
func (gm *GitModule) GetRefRootCommit(ref string) (string, error) {
	args := []string{"rev-list", "--max-parents=0", ref}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		outStr := out.String()
		if strings.Contains(outStr, "unknown revision or path") {
			return "", plumbing.ErrRefNotFound
		}
		return "", errors.Wrap(err, outStr)
	}
	return strings.TrimSpace(out.String()), nil
}

var ErrGitVarNotFound = fmt.Errorf("variable not found")

// Var returns the value of git's logical variables
func (gm *GitModule) Var(name string) (string, error) {
	args := []string{"var", name}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out, err := cmd.Output()
	if err != nil {
		return "", ErrGitVarNotFound
	}
	return strings.TrimSpace(string(out)), nil
}

// ExpandShortHash expands a short hash into its longer variant
func (gm *GitModule) ExpandShortHash(hash string) (string, error) {
	args := []string{"rev-parse", hash}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// RefFetch fetches a remote branch into a local branch
func (gm *GitModule) RefFetch(params remotetypes.RefFetchArgs) error {
	args := []string{"fetch", params.Remote, fmt.Sprintf("%s:%s", params.RemoteRef, params.LocalRef)}
	if params.Verbose {
		args = append(args, "-v")
	}
	if params.Force {
		args = append(args, "-f")
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return errors.Wrap(cmd.Run(), "failed to fetch")
}

// GC performs garbage collection
func (gm *GitModule) GC(pruneExpire ...string) error {
	args := []string{"gc"}
	if len(pruneExpire) > 0 {
		args = append(args, "--prune="+pruneExpire[0])
	}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	errBuf := bytes.NewBuffer(nil)
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errBuf.String())
	}
	return nil
}

// Size returns the size of all packed, loose and garbage objects
func (gm *GitModule) Size() (size float64, err error) {
	args := []string{"count-objects", "-vH"}
	cmd := exec.Command(gm.gitBinPath, args...)
	cmd.Dir = gm.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, errors.Wrap(err, string(out))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "size") {
			size += cast.ToFloat64(strings.Fields(scanner.Text())[1]) * 1024
		}
	}

	return
}
