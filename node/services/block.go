package services

import (
	core_types "github.com/tendermint/tendermint/rpc/core/types"
)

// GetBlock fetches a block at the given height
func (s *NodeService) GetBlock(height int64) (*core_types.ResultBlock, error) {
	var h = &height
	if *h == 0 {
		h = nil
	}
	return s.tmrpc.Block(h)
}
