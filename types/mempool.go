package types

import (
	"github.com/tendermint/tendermint/mempool"
)


// Mempool describes a transaction pool for ordering transactions that will be
// added to a future block.
type Mempool interface {
	mempool.Mempool
	
	// Add attempts to add a transaction to the pool
	Add(tx BaseTx) error 
}
