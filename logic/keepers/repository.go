package keepers

import (
	"fmt"
	"strconv"

	"github.com/make-os/kit/storage"
	"github.com/make-os/kit/storage/common"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"

	"github.com/make-os/kit/pkgs/tree"
	"github.com/pkg/errors"
)

// RepoKeeper manages repository state.
type RepoKeeper struct {
	state *tree.SafeTree
	db    storagetypes.Tx
}

// NewRepoKeeper creates an instance of RepoKeeper
func NewRepoKeeper(state *tree.SafeTree, db storagetypes.Tx) *RepoKeeper {
	return &RepoKeeper{state: state, db: db}
}

// Get implements RepoKeeper
func (rk *RepoKeeper) Get(name string, blockNum ...uint64) *state.Repository {

	repo := rk.GetNoPopulate(name, blockNum...)

	// For each proposal in the repo, fetch their config from the version of the
	// repo where they first appeared.
	stateVersion := rk.state.Version()
	err := repo.Proposals.ForEach(func(prop *state.RepoProposal, id string) error {
		if prop.Height.UInt64() == uint64(stateVersion) {
			prop.Config = repo.Config.Gov
			return nil
		}
		propParent := rk.GetNoPopulate(name, prop.Height.UInt64())
		if propParent.IsNil() {
			return fmt.Errorf("failed to get repo version of proposal (%s)", id)
		}
		prop.Config = propParent.Config.Gov
		return nil
	})
	if err != nil {
		panic(err)
	}

	return repo
}

// GetNoPopulate implements RepoKeeper
func (rk *RepoKeeper) GetNoPopulate(name string, blockNum ...uint64) *state.Repository {

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
		_, bs = rk.state.GetVersioned(key, int64(version))
	} else {
		_, bs = rk.state.Get(key)
	}

	// If we don't find the repo, we return an empty repository.
	if bs == nil {
		return state.BareRepository()
	}

	// Otherwise, we decode the repo bytes to types.Repository
	repo, err := state.NewRepositoryFromBytes(bs)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode repo"))
	}

	return repo
}

// Update implements RepoKeeper
func (rk *RepoKeeper) Update(name string, upd *state.Repository) {
	rk.state.Set(MakeRepoKey(name), upd.Bytes())
}

// IndexProposalVote implements RepoKeeper
func (rk *RepoKeeper) IndexProposalVote(name, propID, voterAddr string, vote int) error {
	key := MakeRepoProposalVoteKey(name, propID, voterAddr)
	rec := common.NewFromKeyValue(key, []byte(fmt.Sprintf("%d", vote)))
	if err := rk.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index proposal vote")
	}

	return nil
}

// GetProposalVote implements RepoKeeper
func (rk *RepoKeeper) GetProposalVote(
	name, propID,
	voterAddr string) (vote int, found bool, err error) {

	key := MakeRepoProposalVoteKey(name, propID, voterAddr)
	rec, err := rk.db.Get(key)
	if err != nil {
		if err != storage.ErrRecordNotFound {
			return 0, false, err
		}
		return 0, false, nil
	}

	vote, _ = strconv.Atoi(string(rec.Value))

	return vote, true, nil
}

// IndexProposalEnd implements RepoKeeper
func (rk *RepoKeeper) IndexProposalEnd(name, propID string, endHeight uint64) error {
	key := MakeRepoProposalEndIndexKey(name, propID, endHeight)
	rec := common.NewFromKeyValue(key, []byte("0"))
	if err := rk.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index proposal end")
	}
	return nil
}

// GetProposalsEndingAt implements RepoKeeper
func (rk *RepoKeeper) GetProposalsEndingAt(height uint64) []*core.EndingProposals {
	key := MakeQueryKeyRepoProposalAtEndHeight(height)
	var res []*core.EndingProposals
	rk.db.NewTx(true, true).Iterate(key, true, func(rec *common.Record) bool {
		prefixes := common.SplitPrefix(rec.GetKey())
		res = append(res, &core.EndingProposals{
			RepoName:   string(prefixes[2]),
			ProposalID: string(prefixes[3]),
			EndHeight:  height,
		})
		return false
	})
	return res
}

// MarkProposalAsClosed implements RepoKeeper
func (rk *RepoKeeper) MarkProposalAsClosed(name, propID string) error {
	key := MakeClosedProposalKey(name, propID)
	rec := common.NewFromKeyValue(key, []byte("0"))
	if err := rk.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to mark proposal as closed")
	}
	return nil
}

// IsProposalClosed implements RepoKeeper
func (rk *RepoKeeper) IsProposalClosed(name, propID string) (bool, error) {
	key := MakeClosedProposalKey(name, propID)
	_, err := rk.db.Get(key)
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// IndexRepoCreatedByAddress implements RepoKeeper
func (rk *RepoKeeper) IndexRepoCreatedByAddress(address []byte, repoName string) error {
	key := MakeAddressRepoPairKey(address, repoName)
	rec := common.NewFromKeyValue(key, []byte("0"))
	if err := rk.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index address and repo name pair")
	}
	return nil
}

// GetReposCreatedByAddress implements RepoKeeper
func (rk *RepoKeeper) GetReposCreatedByAddress(address []byte) (res []string, err error) {
	key := MakeQueryAddressRepoPairKey(address)
	res = []string{}
	rk.db.NewTx(true, true).Iterate(key, true, func(rec *common.Record) bool {
		res = append(res, string(rec.Key))
		return false
	})
	return res, nil
}
