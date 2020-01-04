package keepers

import (
	"github.com/makeos/mosdef/storage/tree"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/types"
)

// RepoKeeper manages repository state.
type RepoKeeper struct {
	state *tree.SafeTree
}

// NewRepoKeeper creates an instance of RepoKeeper
func NewRepoKeeper(state *tree.SafeTree) *RepoKeeper {
	return &RepoKeeper{state: state}
}

// GetRepo finds a repository by name.
//
// ARGS:
// name: The name of the repository to find.
// blockNum: The target block to query (Optional. Default: latest)
//
// CONTRACT: It returns an empty Repository if no repo is found.
func (a *RepoKeeper) GetRepo(name string, blockNum ...uint64) *types.Repository {

	// Get version is provided
	var version uint64
	if len(blockNum) > 0 && blockNum[0] > 0 {
		version = blockNum[0]
	}

	// Query the repo by key. If version is provided,
	// we do a versioned query, otherwise we query the latest.
	key := MakeRepoKey(name)
	var bs []byte
	if version != 0 {
		_, bs = a.state.GetVersioned(key, int64(version))
	} else {
		_, bs = a.state.Get(key)
	}

	// If we don't find the repo, we return an empty repository.
	if bs == nil {
		return types.BareRepository()
	}

	// Otherwise, we decode the repo bytes to types.Repository
	repo, err := types.NewRepositoryFromBytes(bs)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode repo byte slice"))
	}

	return repo
}

// Update sets a new object at the given name.
//
// ARGS:
// name: The name of the repository to update
// udp: The updated repository object to replace the existing object.
func (a *RepoKeeper) Update(name string, upd *types.Repository) {
	a.state.Set(MakeRepoKey(name), upd.Bytes())
}
