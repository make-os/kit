package keepers

import (
	"fmt"
	"sync"

	"github.com/themakeos/lobe/storage"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/util"
)

// ErrBlockInfoNotFound means the block info was not found
var ErrBlockInfoNotFound = fmt.Errorf("block info not found")

// SystemKeeper stores system information such as
// app states, commit history and more.
type SystemKeeper struct {
	db storage.Tx

	gmx       *sync.RWMutex
	lastSaved *core.BlockInfo
}

// NewSystemKeeper creates an instance of SystemKeeper
func NewSystemKeeper(db storage.Tx) *SystemKeeper {
	return &SystemKeeper{db: db, gmx: &sync.RWMutex{}}
}

// SaveBlockInfo saves a committed block information.
// Indexes the saved block info for faster future retrieval so
// that GetLastBlockInfo will not refetch
func (s *SystemKeeper) SaveBlockInfo(info *core.BlockInfo) error {
	data := util.ToBytes(info)
	record := storage.NewFromKeyValue(MakeKeyBlockInfo(info.Height.Int64()), data)

	s.gmx.Lock()
	s.lastSaved = info
	s.gmx.Unlock()

	return s.db.Put(record)
}

// GetLastBlockInfo returns information about the last committed block.
func (s *SystemKeeper) GetLastBlockInfo() (*core.BlockInfo, error) {

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

	var blockInfo core.BlockInfo
	if err := rec.Scan(&blockInfo); err != nil {
		return nil, err
	}

	return &blockInfo, nil
}

// GetBlockInfo returns block information at a given height
func (s *SystemKeeper) GetBlockInfo(height int64) (*core.BlockInfo, error) {
	rec, err := s.db.Get(MakeKeyBlockInfo(height))
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return nil, ErrBlockInfoNotFound
		}
		return nil, err
	}

	var blockInfo core.BlockInfo
	if err := rec.Scan(&blockInfo); err != nil {
		return nil, err
	}

	return &blockInfo, nil
}

// SetLastRepoObjectsSyncHeight sets the last block that was processed by the repo
// object synchronizer
func (s *SystemKeeper) SetLastRepoObjectsSyncHeight(height uint64) error {
	data := util.ToBytes(height)
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

// SetHelmRepo sets the governing repository of the network
func (s *SystemKeeper) SetHelmRepo(name string) error {
	data := []byte(name)
	record := storage.NewFromKeyValue(MakeKeyHelmRepo(), data)
	return s.db.Put(record)
}

// GetHelmRepo gets the governing repository of the network
func (s *SystemKeeper) GetHelmRepo() (string, error) {
	record, err := s.db.Get(MakeKeyHelmRepo())
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return string(record.Value), nil
}
