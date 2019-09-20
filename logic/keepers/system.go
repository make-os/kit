package keepers

import (
	"fmt"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// ErrBlockInfoNotFound means the last block info was not found
var ErrBlockInfoNotFound = fmt.Errorf("last block info not found")

// SystemKeeper stores system information such as
// app states, commit history and more.
type SystemKeeper struct {
	db storage.Engine
}

// NewSystemKeeper creates an instance of SystemKeeper
func NewSystemKeeper(db storage.Engine) *SystemKeeper {
	return &SystemKeeper{db: db}
}

// SaveBlockInfo saves a committed block information
func (s *SystemKeeper) SaveBlockInfo(info *types.BlockInfo) error {
	data := util.ObjectToBytes(info)
	record := storage.NewFromKeyValue(MakeKeyBlockInfo(info.Height), data)
	return s.db.Put(record)
}

// GetLastBlockInfo returns information about the last committed block
func (s *SystemKeeper) GetLastBlockInfo() (*types.BlockInfo, error) {
	var rec *storage.Record

	s.db.Iterate(MakeQueryKeyBlockInfo(), false, func(r *storage.Record) bool {
		rec = r
		return true
	})
	if rec == nil {
		return nil, ErrBlockInfoNotFound
	}

	var blockInfo types.BlockInfo
	rec.Scan(&blockInfo)

	return &blockInfo, nil
}
