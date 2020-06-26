package types

import (
	"context"

	"github.com/libp2p/go-libp2p-core/network"
	"gitlab.com/makeos/mosdef/util/io"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// ObjectStreamer provides an interface for announcing objects and fetching
// various object types from the underlying DHT network.
type ObjectStreamer interface {

	// Announce announces an object's hash
	Announce(hash []byte, doneCB func(error))

	// GetCommit gets a single commit by hash.
	// It returns the packfile, the commit object and error.
	GetCommit(ctx context.Context, repo string, hash []byte) (packfile io.ReadSeekerCloser,
		commit *object.Commit, err error)

	// GetCommitWithAncestors gets a commit and also its ancestors that do not exist
	// in the local repository. It will stop fetching ancestors when it finds
	// an ancestor matching the given end hash.
	// If EndHash is true, it is expected that EndHash commit must exist locally.
	// Packfiles returned are expected to be closed by the caller.
	// If ResultCB is set, packfiles will be passed to the callback and not returned.
	// If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
	// with a nil error.
	GetCommitWithAncestors(ctx context.Context, args GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error)

	// GetTaggedCommitWithAncestors gets the ancestors of the commit pointed by the given tag that
	// do not exist in the local repository.
	// - If EndHash is set, it must be an already existing tag pointing to a commit.
	// - If EndHash is set, it will stop fetching ancestors when it finds an
	//   ancestor matching the commit pointed by the end hash tag.
	// - Packfiles returned are expected to be closed by the caller.
	// - If ResultCB is set, packfiles will be passed to the callback as soon as they are received.
	// - If ResultCB is set, empty slice will be returned by the method.
	// - If ResultCB returns an error, the method exits with that error. Use ErrExit to exit
	//   with a nil error.
	GetTaggedCommitWithAncestors(ctx context.Context, args GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error)

	// GetTag gets a single annotated tag by hash.
	// It returns the packfile, the tag object and error.
	GetTag(ctx context.Context, repo string, hash []byte) (packfile io.ReadSeekerCloser, tag *object.Tag, err error)

	// OnRequest handles incoming object requests
	OnRequest(s network.Stream) (success bool, err error)
}

// GetAncestorArgs contain arguments for GetAncestors method
type GetAncestorArgs struct {

	// RepoName is the target repository to query commits from.
	RepoName string

	// StartHash is the hash of the object to start from
	StartHash []byte

	// EndHash is the hash of the object that indicates the end of the query.
	// If provided, it must exist on the local repository of the caller.
	EndHash []byte

	// ExcludeEndCommit when true, indicates that the end commit should not be fetched.
	ExcludeEndCommit bool

	// GitBinPath is the path to the git binary
	GitBinPath string

	// ReposDir is the root directory containing all repositories
	ReposDir string

	// ResultCB is a callback used for collecting packfiles as they are fetched.
	// If not set, all packfile results a collected and return at the end of the query.
	// hash is the object hash of the object that owns the packfile.
	ResultCB func(packfile io.ReadSeekerCloser, hash string) error
}
