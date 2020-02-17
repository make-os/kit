package repo

import (
	"fmt"
	"strings"

	"github.com/makeos/mosdef/types"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// ErrRepoNotFound means a repo was not found on the local storage
var ErrRepoNotFound = fmt.Errorf("repo not found")

func getKVOpt(key string, options []types.KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

func matchOpt(val string) types.KVOption {
	return types.KVOption{Key: "match", Value: val}
}

func changesOpt(ch *types.Changes) types.KVOption {
	return types.KVOption{Key: "changes", Value: ch}
}

// MakeRepoObjectDHTKey returns a key for announcing a repository object
func MakeRepoObjectDHTKey(repoName, hash string) string {
	return fmt.Sprintf("%s/%s", repoName, hash)
}

// ParseRepoObjectDHTKey parses a dht key for finding repository objects
func ParseRepoObjectDHTKey(key string) (repoName string, hash string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo object dht key")
	}
	return parts[0], parts[1], nil
}

func checkEvtArgs(args []interface{}) error {
	if len(args) != 2 {
		panic("invalid number of arguments")
	}

	if args[0] == nil {
		return nil
	}

	err, ok := args[0].(error)
	if !ok {
		panic("invalid type at evt.Arg[0]")
	}

	return err
}

// WrappedCommit wraps a go-git commit to ensure it conforms to types.Commit
type WrappedCommit struct {
	*object.Commit
}

// Parent returns the ith parent of a commit.
func (c *WrappedCommit) Parent(i int) (types.Commit, error) {
	parent, err := c.Commit.Parent(i)
	if err != nil {
		return nil, err
	}
	return &WrappedCommit{parent}, nil
}

// GetTreeHash returns the hash of the root tree of the commit
func (c *WrappedCommit) GetTreeHash() plumbing.Hash {
	return c.TreeHash
}

// GetAuthor returns the original author of the commit.
func (c *WrappedCommit) GetAuthor() *object.Signature {
	return &c.Author
}

// GetCommitter returns the one performing the commit, might be different from Author
func (c *WrappedCommit) GetCommitter() *object.Signature {
	return &c.Committer
}

// GetHash returns the hash of the commit object
func (c *WrappedCommit) GetHash() plumbing.Hash {
	return c.Hash
}
