package repo

import (
	"fmt"
	"regexp"
	"strings"

	"gitlab.com/makeos/mosdef/types/core"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func getKVOpt(key string, options []core.KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

func matchOpt(val string) core.KVOption {
	return core.KVOption{Key: "match", Value: val}
}

func changesOpt(ch *core.Changes) core.KVOption {
	return core.KVOption{Key: "changes", Value: ch}
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

// isBranch checks whether a reference name indicates a branch
func isBranch(refname string) bool {
	return plumbing.ReferenceName(refname).IsBranch()
}

// isIssueBranch checks whether a branch is an issue branch
func isIssueBranch(name string) bool {
	return regexp.MustCompile("^refs/heads/issues/.*").MatchString(name)
}

// isReference checks the given name is a reference path or full reference name
func isReference(refname string) bool {
	m, _ := regexp.MatchString("^refs/(heads|tags|notes)((/[a-z0-9_-]+)+)?$", refname)
	return m
}

// isTag checks whether a reference name indicates a tag
func isTag(refname string) bool {
	return plumbing.ReferenceName(refname).IsTag()
}

// isNote checks whether a reference name indicates a tag
func isNote(refname string) bool {
	return plumbing.ReferenceName(refname).IsNote()
}

// isZeroHash checks whether a given hash is a zero git hash
func isZeroHash(h string) bool {
	return h == plumbing.ZeroHash.String()
}

// WrappedCommit wraps a go-git commit to ensure it conforms to types.WrappedCommit
type WrappedCommit struct {
	*object.Commit
}

// wrapCommit creates a WrappedCommit that wraps a go-git commit object
func wrapCommit(gc *object.Commit) *WrappedCommit {
	return &WrappedCommit{gc}
}

// Parent returns the ith parent of a commit.
func (c *WrappedCommit) Parent(i int) (core.Commit, error) {
	parent, err := c.Commit.Parent(i)
	if err != nil {
		return nil, err
	}
	return &WrappedCommit{parent}, nil
}

// IsParent checks whether the specified hash is a parent of the commit
func (c *WrappedCommit) IsParent(hash string) (bool, core.Commit) {
	for i := 0; i < c.NumParents(); i++ {
		if parent, _ := c.Parent(i); parent != nil && parent.GetHash().String() == hash {
			return true, parent
		}
	}
	return false, nil
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

// GetTree returns the tree from the commit
func (c *WrappedCommit) GetTree() (*object.Tree, error) {
	return c.Tree()
}
