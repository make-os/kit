package core

import (
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/mempool"
	tmtypes "github.com/tendermint/tendermint/types"
)

// Mempool describes a transaction pool for ordering transactions that will be
// added to a future block.
type Mempool interface {

	// CheckTx executes a new transaction against the application to determine
	// its validity and whether it should be added to the mempool.
	CheckTx(tx tmtypes.Tx, callback func(*abci.Response)) error

	// CheckTxWithInfo performs the same operation as CheckTx, but with extra
	// meta data about the tx.
	// Currently this metadata is the peer who sent it, used to prevent the tx
	// from being gossiped back to them.
	CheckTxWithInfo(tx tmtypes.Tx, callback func(*abci.Response), txInfo mempool.TxInfo) error

	// ReapMaxBytesMaxGas reaps transactions from the mempool up to maxBytes
	// bytes total with the condition that the total gasWanted must be less than
	// maxGas.
	// If both maxes are negative, there is no cap on the size of all returned
	// transactions (~ all available transactions).
	ReapMaxBytesMaxGas(maxBytes, maxGas int64) tmtypes.Txs

	// ReapMaxTxs reaps up to max transactions from the mempool.
	// If max is negative, there is no cap on the size of all returned
	// transactions (~ all available transactions).
	ReapMaxTxs(max int) tmtypes.Txs

	// Lock locks the mempool. The consensus must be able to hold lock to safely update.
	Lock()

	// Unlock unlocks the mempool.
	Unlock()

	// Update informs the mempool that the given txs were committed and can be discarded.
	// NOTE: this should be called *after* block is committed by consensus.
	// NOTE: unsafe; Lock/Unlock must be managed by caller
	Update(blockHeight int64, blockTxs tmtypes.Txs, deliverTxResponses []*abci.ResponseDeliverTx,
		newPreFn mempool.PreCheckFunc, newPostFn mempool.PostCheckFunc) error

	// FlushAppConn flushes the mempool connection to ensure async reqResCb calls are
	// done. E.g. from CheckTx.
	FlushAppConn() error

	// Flush removes all transactions from the mempool and cache
	Flush()

	// TxsAvailable returns a channel which fires once for every height,
	// and only when transactions are available in the mempool.
	// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
	TxsAvailable() <-chan struct{}

	// EnableTxsAvailable initializes the TxsAvailable channel, ensuring it will
	// trigger once every height when transactions are available.
	EnableTxsAvailable()

	// Size returns the number of transactions in the mempool.
	Size() int

	// TxsBytes returns the total size of all txs in the mempool.
	TxsBytes() int64

	// InitWAL creates a directory for the WAL file and opens a file itself.
	InitWAL()

	// CloseWAL closes and discards the underlying WAL file.
	// Any further writes will not be relayed to disk.
	CloseWAL()

	// Register attempts to add a transaction to the pool
	Add(tx types.BaseTx) (bool, error)
}

// MempoolReactor provides access to mempool operations
type MempoolReactor interface {
	GetPoolSize() *PoolSizeInfo
	GetTop(n int) []types.BaseTx
	AddTx(tx types.BaseTx) (hash util.HexBytes, err error)
	GetTx(hash string) types.BaseTx
}

// PoolSizeInfo describes the transaction byte size an count of the tx pool
type PoolSizeInfo struct {
	TotalTxSize int64 `json:"size"`
	TxCount     int   `json:"count"`
	CacheSize   int   `json:"cache"`
}
