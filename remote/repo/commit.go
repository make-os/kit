package repo

import (
	"gitlab.com/makeos/mosdef/remote/types"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// WrappedCommit wraps a go-git commit to ensure it conforms to types.WrappedCommit
type WrappedCommit struct {
	*object.Commit
}

// wrapCommit creates a WrappedCommit that wraps a go-git commit object
func WrapCommit(gc *object.Commit) *WrappedCommit {
	return &WrappedCommit{gc}
}

// UnWrap returns the underlying commit object
func (c *WrappedCommit) UnWrap() *object.Commit {
	return c.Commit
}

// Parent returns the ith parent of a commit.
func (c *WrappedCommit) Parent(i int) (types.Commit, error) {
	parent, err := c.Commit.Parent(i)
	if err != nil {
		return nil, err
	}
	return &WrappedCommit{parent}, nil
}

// IsParent checks whether the specified hash is a parent of the commit
func (c *WrappedCommit) IsParent(hash string) (bool, types.Commit) {
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
