package plumbing

import "gopkg.in/src-d/go-git.v4/plumbing"

// MakeCommitHash creates and returns a commit hash from the specified data
func MakeCommitHash(data string) plumbing.Hash {
	return plumbing.ComputeHash(plumbing.CommitObject, []byte(data))
}

// isZeroHash checks whether a given hash is a zero git hash
func IsZeroHash(h string) bool {
	return h == plumbing.ZeroHash.String()
}
