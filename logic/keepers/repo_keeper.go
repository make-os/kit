package keepers

import (
	"fmt"
	"strconv"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/types"
)

// RepoKeeper manages repository state.
type RepoKeeper struct {
	state *tree.SafeTree
	db    storage.Tx
}

// NewRepoKeeper creates an instance of RepoKeeper
func NewRepoKeeper(state *tree.SafeTree, db storage.Tx) *RepoKeeper {
	return &RepoKeeper{state: state, db: db}
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

// IndexProposalVote indexes a proposal vote.
//
// ARGS:
// name: The name of the repository
// propID: The target proposal
// voterAddr: The address of the voter
// vote: Indicates the vote choice
func (a *RepoKeeper) IndexProposalVote(name, propID, voterAddr string, vote int) error {
	key := MakeRepoProposalVoteKey(name, propID, voterAddr)
	rec := storage.NewFromKeyValue(key, []byte(fmt.Sprintf("%d", vote)))
	if err := a.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index proposal vote")
	}

	return nil
}

// GetProposalVote returns the vote choice of the
// given voter for the given proposal
//
// ARGS:
// name: The name of the repository
// propID: The target proposal
// voterAddr: The address of the voter
func (a *RepoKeeper) GetProposalVote(
	name, propID,
	voterAddr string) (vote int, found bool, err error) {

	key := MakeRepoProposalVoteKey(name, propID, voterAddr)
	rec, err := a.db.Get(key)
	if err != nil {
		if err != storage.ErrRecordNotFound {
			return 0, false, err
		}
		return 0, false, nil
	}

	vote, _ = strconv.Atoi(string(rec.Value))

	return vote, true, nil
}

// IndexProposalEnd indexes a proposal by its end height so it can be
// tracked and finalized at the given height
func (a *RepoKeeper) IndexProposalEnd(name, propID string, endHeight uint64) error {
	key := MakeRepoProposalEndIndexKey(name, propID, endHeight)
	rec := storage.NewFromKeyValue(key, []byte("0"))
	if err := a.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index proposal end")
	}
	return nil
}

// GetProposalsEndingAt finds repo proposals ending at the given height
func (a *RepoKeeper) GetProposalsEndingAt(height uint64) []*types.EndingProposals {
	key := MakeQueryKeyRepoProposalAtEndHeight(height)
	var res = []*types.EndingProposals{}
	a.db.Iterate(key, true, func(rec *storage.Record) bool {
		prefixes := storage.SplitPrefix(rec.GetKey())
		res = append(res, &types.EndingProposals{
			RepoName:   string(prefixes[2]),
			ProposalID: string(prefixes[3]),
			EndHeight:  height,
		})
		return false
	})
	return res
}
