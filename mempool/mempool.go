package mempool

import (
	"context"
	"fmt"
	"sync"
	"time"

	types2 "github.com/make-os/kit/mempool/types"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/validation"

	"github.com/make-os/kit/config"

	"github.com/make-os/kit/types"

	"github.com/make-os/kit/pkgs/logger"

	"github.com/make-os/kit/mempool/pool"
	abci "github.com/tendermint/tendermint/abci/types"
	auto "github.com/tendermint/tendermint/libs/autofile"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	tmtypes "github.com/tendermint/tendermint/types"
)

// Option sets an optional parameter on the mempool.
type Option func(*Mempool)

// Mempool implements mempool.Mempool
type Mempool struct {
	cfg   *config.AppConfig
	logic core.Logic

	proxyMtx     sync.Mutex
	proxyAppConn proxy.AppConnMempool
	pool         *pool.Pool
	preCheck     mempool.PreCheckFunc
	postCheck    mempool.PostCheckFunc
	validateTx   validation.ValidateTxFunc

	// A log of mempool txs
	wal *auto.AutoFile

	log     logger.Logger
	metrics *mempool.Metrics
}

// InitWAL implements mempool.Mempool
func (mp *Mempool) InitWAL() error {
	return nil
}

// NewMempool creates an instance of Mempool
func NewMempool(cfg *config.AppConfig, logic core.Logic) *Mempool {
	return &Mempool{
		cfg:        cfg,
		pool:       pool.New(cfg.Mempool.Size, logic, cfg.G().Bus),
		logic:      logic,
		log:        cfg.G().Log.Module("mempool"),
		validateTx: validation.ValidateTx,
	}
}

// SetProxyApp sets the proxy app connection for accessing
// ABCI app operations required by the mempool
func (mp *Mempool) SetProxyApp(proxyApp proxy.AppConnMempool) {
	mp.proxyAppConn = proxyApp
	mp.proxyAppConn.SetResponseCallback(mp.globalCb)
}

// CheckTx executes a new transaction against the application to determine
// its validity and whether it should be added to the mempool.
func (mp *Mempool) CheckTx(tx tmtypes.Tx, callback func(*abci.Response), txInfo mempool.TxInfo) error {
	panic("not implemented")
}

// Add attempts to add a transaction to the pool
func (mp *Mempool) Add(tx types.BaseTx) (bool, error) {

	// Check if there is capacity for another transaction.
	if err := mp.checkCapacity(tx); err != nil {
		return false, err
	}

	// Set TxMetaKeyAllowNonceGap: Causes tx checks to allow nonce with > 1 difference
	// than the sender's current account nonce.
	tx.SetMeta(types.TxMetaKeyAllowNonceGap, true)

	// Check the transaction
	if err := mp.validateTx(tx, -1, mp.logic); err != nil {
		mp.cfg.G().Bus.Emit(types2.EvtMempoolTxRejected, err, tx)
		mp.log.Debug("Rejected an invalid transaction", "Reason", err.Error())
		return false, err
	}

	// Add valid transaction to the pool
	addedToPool, err := mp.pool.Put(tx)
	if err != nil {
		mp.cfg.G().Bus.Emit(types2.EvtMempoolTxRejected, err, tx)
		return false, err
	}

	if addedToPool {
		mp.log.Info("Added a new transaction to the pool",
			"Hash", tx.GetHash(),
			"PoolSize", mp.Size())
	} else {
		mp.log.Info("Added a new transaction to the cache",
			"Hash", tx.GetHash(),
			"CacheSize", mp.CacheSize())
	}

	return addedToPool, nil
}

// checkCapacity checks whether there is enough pool capacity for the new transaction
func (mp *Mempool) checkCapacity(tx types.BaseTx) error {
	var (
		memSize  = mp.Size()
		txsBytes = mp.TxsBytes()
		txSize   = len(tx.Bytes())
	)

	// Check whether the pool has sufficient capacity
	// to accommodate this new transaction
	if memSize >= mp.cfg.Mempool.Size || int64(txSize)+txsBytes > mp.cfg.Mempool.MaxTxsSize {
		msg := "mempool is full: number of txs %d (max: %d), total txs bytes %d (max: %d)"
		return fmt.Errorf(msg, memSize, mp.cfg.Mempool.Size, txsBytes, mp.cfg.Mempool.MaxTxsSize)
	}

	// The size of the corresponding amino-encoded TxMessage
	// can't be larger than the maxMsgSize, otherwise we can't
	// relay it to peers.
	if txSize > mp.cfg.Mempool.MaxTxSize {
		return fmt.Errorf("transaction is too large. Max size is %d, but got %d", mp.cfg.Mempool.MaxTxSize, txSize)
	}

	return nil
}

// onTxCheckFinished returns a callback function that is called
// when the abci app checks the transaction.
// The argument externalCb is a callback that is called after onTxCheckFinished
// has finished its operations. It used by external callers to initiate
// other operations that need to be executed after onTxCheckFinished finishes.
func (mp *Mempool) onTxCheckFinished(_ []byte, _ uint16, _ func(*abci.Response)) func(res *abci.Response) {
	return func(res *abci.Response) {
		panic("not implemented")
	}
}

// ReapMaxBytesMaxGas reaps transactions from the mempool up to maxBytes
// bytes total. If both maxBytes are negative, there is no cap on the
// size of all returned transactions.
// NOTE: maxGas is ignored since this mempool does not apply the concept of gas
func (mp *Mempool) ReapMaxBytesMaxGas(maxBytes, maxGas int64) tmtypes.Txs {
	mp.proxyMtx.Lock()
	defer mp.proxyMtx.Unlock()

	var totalBytes int64
	var ignoredTx []types.BaseTx
	var numValTicketTxReaped int
	var selectedProposals = make(map[string]map[string]struct{})
	var txs = make([]tmtypes.Tx, 0, mp.pool.Size())

	for {

		// Fetch a transaction. Exit if nil is returned.
		memTx := mp.pool.Head()
		if memTx == nil {
			break
		}

		// If tx is a proposal creator, we need to ensure we do not allow same
		// proposal ID for same repo to be selected. If we find a matching proposal ID
		// targeting same repository, we have to put the transaction back in the pool.
		if proposal, ok := memTx.(types.ProposalTx); ok {
			repoName := proposal.GetProposalRepoName()
			selected := selectedProposals[repoName]
			if selected == nil {
				selectedProposals[repoName] = map[string]struct{}{proposal.GetProposalID(): {}}
			} else if _, ok := selected[proposal.GetProposalID()]; ok {
				ignoredTx = append(ignoredTx, memTx)
				continue
			}
		}

		// If tx is a validator ticket, we need to ensure we do not select more than
		// the required max validator tickets. If we have reached the limit we have to
		// put the transaction back in the pool.
		if memTx.Is(txns.TxTypeValidatorTicket) {
			if numValTicketTxReaped == params.MaxValTicketsPerBlock {
				ignoredTx = append(ignoredTx, memTx)
				continue
			}
			numValTicketTxReaped++
		}

		// Check total size requirement
		txBs := memTx.Bytes()
		txSize := len(txBs)
		if maxBytes > -1 && totalBytes+int64(txSize) > maxBytes {
			return txs
		}
		totalBytes += int64(txSize)

		// Add tx to the reaped list
		txs = append(txs, txBs)
	}

	// Flush ignored tx back to the pool
	for _, tx := range ignoredTx {
		mp.pool.Put(tx)
	}

	return txs
}

// ReapMaxTxs reaps up to max transactions from the mempool.
// If max is negative, there is no cap on the size of all returned
// transactions (~ all available transactions).
func (mp *Mempool) ReapMaxTxs(max int) tmtypes.Txs {
	panic("not implemented")
}

// Lock locks the mempool. The consensus must be able to hold lock to safely update.
func (mp *Mempool) Lock() {
	mp.proxyMtx.Lock()
}

// Unlock unlocks the mempool.
func (mp *Mempool) Unlock() {
	mp.proxyMtx.Unlock()
}

// recheckTxs rechecks transactions in the mempool and will remove invalidated transactions.
// This is called when a new block has been committed.
func (mp *Mempool) recheckTxs() {
	mp.pool.Find(func(tx types.BaseTx, u util.String, timeAdded time.Time) bool {
		if err := mp.validateTx(tx, -1, mp.logic); err != nil {
			mp.pool.Remove(tx)
			mp.log.Debug("Removed invalidated transaction", "Hash", tx.GetHash())
		}
		return false
	})
}

// Update informs the mempool that the given txs were committed and can be discarded.
// NOTE: this should be called *after* block is committed by consensus.
// NOTE: unsafe; Lock/Unlock must be managed by caller
func (mp *Mempool) Update(blockHeight int64, txs tmtypes.Txs,
	responses []*abci.ResponseDeliverTx, _ mempool.PreCheckFunc, _ mempool.PostCheckFunc) error {

	for i, txBs := range txs {

		// Decode the transaction
		tx, err := txns.DecodeTx(txBs)
		if err != nil {
			return err
		}

		// Remove it from the pool
		mp.pool.Remove(tx)
		mp.cfg.G().Bus.Emit(types2.EvtMempoolTxRemoved, nil, tx)

		// If the tx was processed into a block, emit event
		if len(responses) > 0 && responses[i].GetCode() == abci.CodeTypeOK {
			mp.cfg.G().Bus.Emit(types2.EvtMempoolTxCommitted, nil, tx)
		}
	}

	// Recheck existing transactions in the pool
	mp.recheckTxs()

	return nil
}

// Flush removes all transactions from the mempool and cache
func (mp *Mempool) Flush() {
	mp.pool.Flush()
}

// TxsAvailable returns a channel which fires once for every height,
// and only when transactions are available in the mempool.
// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
func (mp *Mempool) TxsAvailable() <-chan struct{} {
	return nil
}

// EnableTxsAvailable initializes the TxsAvailable channel, ensuring it will
// trigger once every height when transactions are available.
func (mp *Mempool) EnableTxsAvailable() {

}

// Size returns the number of transactions in the mempool.
func (mp *Mempool) Size() int {
	return mp.pool.Size()
}

// CacheSize returns the number of transactions in the mempool cache.
func (mp *Mempool) CacheSize() int {
	return mp.pool.CacheSize()
}

// TxsBytes returns the total size of all txs in the mempool.
func (mp *Mempool) TxsBytes() int64 {
	return mp.pool.ByteSize()
}

func (mp *Mempool) globalCb(req *abci.Request, res *abci.Response) {}

// CloseWAL closes and discards the underlying WAL file.
// Any further writes will not be relayed to disk.
func (mp *Mempool) CloseWAL() {}

// FlushAppConn flushes the mempool connection to ensure async reqResCb calls are
// done. E.g. from CheckTx.
func (mp *Mempool) FlushAppConn() error {
	return mp.proxyAppConn.FlushSync(context.Background())
}
