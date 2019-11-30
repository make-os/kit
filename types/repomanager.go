package types

import (
	"context"

	"gopkg.in/src-d/go-git.v4/storage"
)

// RepoManager describes an interface for servicing
// and managing git repositories
type RepoManager interface {

	// Start starts the server
	Start()

	// Wait can be used by the caller to wait
	// till the server terminates
	Wait()

	// Stop shutsdown the server
	Stop(ctx context.Context)

	// CreateRepository creates a local git repository
	CreateRepository(name string) error
}

// Repo represents a repository
type Repo interface {
	GitOperations
	storage.Storer
	RepoDBOps
}

// GitOperations describes operations that read and alter a repository
type GitOperations interface {
	// RefDelete executes `git update-ref -d <refname>` to delete a reference
	RefDelete(refname string) error

	// RefUpdate executes `git update-ref <refname> <commit hash>` to update/create a reference
	RefUpdate(refname, commitHash string) error

	// TagDelete executes `git tag -d <tagname>` to delete a tag
	TagDelete(tagname string) error

	// RefGet returns the hash content of a reference.
	// Returns ErrRefNotFound if ref does not exist
	RefGet(refname string) (string, error)

	// GetRecentCommit gets the hash of the recent commit
	// Returns ErrNoCommits if no commits exist
	GetRecentCommit() (string, error)

	// GetHEAD returns the reference stored in HEAD
	// short: When set to true, the full reference name is returned
	GetHEAD(short bool) (string, error)

	// NumCommits gets the number of commits in a branch
	NumCommits(branch string) (int, error)

	// GetConfig finds and returns a config value
	GetConfig(path string) string

	// UpdateRecentCommitMsg updates the recent commit message
	// msg: The commit message which is passed to the command's stdin.
	// signingKey: The signing key
	// env: Optional environment variables to pass to the command.
	UpdateRecentCommitMsg(msg, signingKey string, env ...string) error

	// CreateTagWithMsg an annotated tag
	// args: `git tag` options (NOTE: -a and --file=- are added by default)
	// msg: The tag's message which is passed to the command's stdin.
	// signingKey: The signing key to use
	// env: Optional environment variables to pass to the command.
	CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error

	// ListTreeObjects returns a map containing tree entries (filename: objectname)
	ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string, error)

	// ListTreeObjectsSlice returns a slice containing objects name of tree entries
	ListTreeObjectsSlice(treename string, recursive, showTrees bool, env ...string) ([]string, error)

	// RemoveEntryFromNote removes a note
	RemoveEntryFromNote(notename, objectHash string, env ...string) error

	// AddEntryToNote adds a note
	AddEntryToNote(notename, objectHash, note string, env ...string) error

	// CreateBlob creates a blob object
	CreateBlob(content string) (string, error)
}

// RepoDBOps provides an interface for accessing the database of a repository
type RepoDBOps interface {
	GetCache() *RepoDBOps
}
