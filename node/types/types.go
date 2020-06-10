package types

import (
	"github.com/tendermint/tendermint/types"
)

// BlockGetter describes a structure for getting blocks
type BlockGetter interface {
	// GetBlock returns a tendermint block with the given height.
	GetBlock(height int64) *types.Block

	// GetChainHeight returns the current chain height
	GetChainHeight() int64
}

// Events
const (
	EvtTxPushProcessed = "tx_push_added"
)
