package dht

import (
	"context"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/kit/util/io"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Streamer provides an interface for providing objects
// and fetching various object types from the underlying
// DHT network.
type Streamer interface {
	GetCommit(ctx context.Context, repo string, hash []byte) (packfile io.ReadSeekerCloser, commit *object.Commit, err error)
	GetCommitWithAncestors(ctx context.Context, args GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error)
	GetTaggedCommitWithAncestors(ctx context.Context, args GetAncestorArgs) (packfiles []io.ReadSeekerCloser, err error)
	GetTag(ctx context.Context, repo string, hash []byte) (packfile io.ReadSeekerCloser, tag *object.Tag, err error)
	OnRequest(s network.Stream) (success bool, err error)
	GetProviders(ctx context.Context, repoName string, objectHash []byte) ([]peer.AddrInfo, error)
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
