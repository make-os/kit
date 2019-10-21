package keepers

import (
	"fmt"
	"sync"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// ErrBlockInfoNotFound means the block info was not found
var ErrBlockInfoNotFound = fmt.Errorf("block info not found")

// SystemKeeper stores system information such as
// app states, commit history and more.
type SystemKeeper struct {
	db storage.Functions

	gmx       *sync.RWMutex
	lastSaved *types.BlockInfo
}

// NewSystemKeeper creates an instance of SystemKeeper
func NewSystemKeeper(db storage.Functions) *SystemKeeper {
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

// SetHighestDrandRound sets the highest drand round to r
// only if r is greater than the current highest round.
func (s *SystemKeeper) SetHighestDrandRound(r uint64) error {
	hr, err := s.GetHighestDrandRound()
	if err != nil {
		return err
	}
	if hr > uint64(r) {
		return nil
	}
	rec := storage.NewRecord(MakeHighestDrandRoundKey(), util.EncodeNumber(uint64(r)))
	return s.db.Put(rec)
}

// GetHighestDrandRound returns the highest known drand round
func (s *SystemKeeper) GetHighestDrandRound() (uint64, error) {
	rec, err := s.db.Get(MakeHighestDrandRoundKey())
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return util.DecodeNumber(rec.Value), nil
}

// GetSecrets fetch secrets from blocks starting from a given
// height back to genesis block. The argument limit puts a
// cap on the number of secrets to be collected. If limit is
// set to 0 or negative number, no limit is applied.
// The argument skip controls how many blocks are skipped.
// Skip is 1 by default. Blocks with an invalid secret or
// no secret are ignored.
func (s *SystemKeeper) GetSecrets(from, limit, skip int64) ([][]byte, error) {
	var secrets [][]byte
	var next = from
	if skip < 1 {
		skip = 1
	}
	for next > 0 {
		bi, err := s.GetBlockInfo(next)
		if err != nil {
			if err != ErrBlockInfoNotFound {
				return nil, err
			}
			next = next - skip
			continue
		}

		if len(bi.EpochSecret) == 0 || bi.InvalidEpochSecret {
			next = next - skip
			continue
		}

		secrets = append(secrets, bi.EpochSecret)
		if limit > 0 && int64(len(secrets)) == limit {
			break
		}

		next = next - skip
	}
	return secrets, nil
}
