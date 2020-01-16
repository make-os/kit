package keepers

import (
	"fmt"
	"sync"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
)

// ErrBlockInfoNotFound means the block info was not found
var ErrBlockInfoNotFound = fmt.Errorf("block info not found")

// SystemKeeper stores system information such as
// app states, commit history and more.
type SystemKeeper struct {
	db storage.Tx

	gmx       *sync.RWMutex
	lastSaved *types.BlockInfo
}

// NewSystemKeeper creates an instance of SystemKeeper
func NewSystemKeeper(db storage.Tx) *SystemKeeper {
	return &SystemKeeper{db: db, gmx: &sync.RWMutex{}}
}

// SaveBlockInfo saves a committed block information.
// Indexes the saved block info for faster future retrieval so
// that GetLastBlockInfo will not refetch
func (s *SystemKeeper) SaveBlockInfo(info *types.BlockInfo) error {
	data := util.ObjectToBytes(info)
	record := storage.NewFromKeyValue(MakeKeyBlockInfo(info.Height), data)

	s.gmx.Lock()
	s.lastSaved = info
	s.gmx.Unlock()

	return s.db.Put(record)
}

// GetLastBlockInfo returns information about the last committed block.
func (s *SystemKeeper) GetLastBlockInfo() (*types.BlockInfo, error) {

	// Retrieve the cached last saved block info if set
	s.gmx.RLock()
	lastSaved := s.lastSaved
	s.gmx.RUnlock()
	if lastSaved != nil {
		return lastSaved, nil
	}

	var rec *storage.Record
	s.db.Iterate(MakeQueryKeyBlockInfo(), false, func(r *storage.Record) bool {
		rec = r
		return true
	})
	if rec == nil {
		return nil, ErrBlockInfoNotFound
	}

	var blockInfo types.BlockInfo
	if err := rec.Scan(&blockInfo); err != nil {
		return nil, err
	}

	return &blockInfo, nil
}

// GetBlockInfo returns block information at a given height
func (s *SystemKeeper) GetBlockInfo(height int64) (*types.BlockInfo, error) {
	rec, err := s.db.Get(MakeKeyBlockInfo(height))
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return nil, ErrBlockInfoNotFound
		}
		return nil, err
	}

	var blockInfo types.BlockInfo
	if err := rec.Scan(&blockInfo); err != nil {
		return nil, err
	}

	return &blockInfo, nil
}

// MarkAsMatured sets the network maturity flag to true.
// The arg maturityHeight is the height maturity was attained.
func (s *SystemKeeper) MarkAsMatured(maturityHeight uint64) error {
	return s.db.Put(storage.
		NewFromKeyValue(MakeNetMaturityKey(), util.EncodeNumber(maturityHeight)))
}

// GetNetMaturityHeight returns the height at which network maturity was attained
func (s *SystemKeeper) GetNetMaturityHeight() (uint64, error) {
	rec, err := s.db.Get(MakeNetMaturityKey())
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return 0, types.ErrImmatureNetwork
		}
		return 0, err
	}
	return util.DecodeNumber(rec.Value), nil
}

// IsMarkedAsMature checks whether there is a net maturity key.
func (s *SystemKeeper) IsMarkedAsMature() (bool, error) {
	_, err := s.db.Get(MakeNetMaturityKey())
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetEpochSeeds traverses the chain's history collecting seeds from every epoch until
// the limit is reached or no more seeds are found.
func (s *SystemKeeper) GetEpochSeeds(startHeight, limit int64) ([][]byte, error) {

	// Determine the end of the epoch where startHeight falls in
	var next = params.GetEndOfEpochOfHeight(startHeight)

	// Skip as much as NumBlocksPerEpoch to reach the next older epoch
	skip := int64(params.NumBlocksPerEpoch)

	var seeds [][]byte
	for next > 0 {
		seedHeight := params.GetSeedHeightInEpochOfHeight(next)
		bi, err := s.GetBlockInfo(seedHeight)
		if err != nil {
			if err != ErrBlockInfoNotFound {
				return nil, err
			}
			next = next - skip
			continue
		}

		// If the seed block does not include a seed, we use the hash of the
		// block before it
		if bi.EpochSeedOutput.IsEmpty() {
			beforeInfo, err := s.GetBlockInfo(seedHeight - 1)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get block before seed block")
			}
			seeds = append(seeds, beforeInfo.Hash)
			next = next - skip
			continue
		}

		seeds = append(seeds, bi.EpochSeedOutput.Bytes())
		if limit > 0 && int64(len(seeds)) == limit {
			break
		}

		next = next - skip
	}
	return seeds, nil
}

// SetLastRepoObjectsSyncHeight sets the last block that was processed by the repo
// object synchronizer
func (s *SystemKeeper) SetLastRepoObjectsSyncHeight(height uint64) error {
	data := util.ObjectToBytes(height)
	record := storage.NewFromKeyValue(MakeKeyRepoSyncherHeight(), data)
	return s.db.Put(record)
}

// GetLastRepoObjectsSyncHeight returns the last block that was processed by the
// repo object synchronizer
func (s *SystemKeeper) GetLastRepoObjectsSyncHeight() (uint64, error) {
	record, err := s.db.Get(MakeKeyRepoSyncherHeight())
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}

	var height uint64
	record.Scan(&height)
	return height, nil
}
