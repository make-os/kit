package mempool

import (
	"fmt"
	"sync"

	"github.com/makeos/mosdef/params"

	"github.com/makeos/mosdef/config"

	t "github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/util/logger"

	"github.com/makeos/mosdef/mempool/pool"
	abci "github.com/tendermint/tendermint/abci/types"
	auto "github.com/tendermint/tendermint/libs/autofile"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// Option sets an optional parameter on the mempool.
type Option func(*Mempool)

// Mempool implements mempool.Mempool
type Mempool struct {
	config *config.EngineConfig

	proxyMtx     sync.Mutex
	proxyAppConn proxy.AppConnMempool
	pool         *pool.Pool
	preCheck     mempool.PreCheckFunc
	postCheck    mempool.PostCheckFunc

	// notify listeners (ie. consensus) when txs are available
	notifiedTxsAvailable bool
	txsAvailable         chan struct{} // fires once for each height, when the mempool is not empty

	// A log of mempool txs
	wal *auto.AutoFile

	// epochSecretGetter is a callback that is used to fetch
	// an epoch secret tx.
	epochSecretGetter func() (t.Tx, error)

	log     logger.Logger
	metrics *mempool.Metrics
}

// NewMempool creates an instance of Mempool
func NewMempool(
	config *config.EngineConfig) *Mempool {
	return &Mempool{
		config: config,
		pool:   pool.New(int64(config.Mempool.Size)),
		log:    config.G().Log.Module("Mempool"),
	}
}

// SetEpochSecretGetter sets the callback function
// that returns an epoch secret transaction
func (mp *Mempool) SetEpochSecretGetter(cb func() (t.Tx, error)) {
	mp.epochSecretGetter = cb
}

// SetProxyApp sets the proxy app connection for accessing
// ABCI app operations required by the mempool
func (mp *Mempool) SetProxyApp(proxyApp proxy.AppConnMempool) {
	mp.proxyAppConn = proxyApp
	mp.proxyAppConn.SetResponseCallback(mp.globalCb)
}

// CheckTx executes a new transaction against the application to determine
// its validity and whether it should be added to the mempool.
func (mp *Mempool) CheckTx(tx types.Tx, callback func(*abci.Response)) error {
	return mp.CheckTxWithInfo(tx, callback, mempool.TxInfo{SenderID: mempool.UnknownPeerID})
}

// CheckTxWithInfo performs the same operation as CheckTx, but with extra
// meta data about the tx.
// Currently this metadata is the peer who sent it, used to prevent the tx
// from being gossiped back to them.
func (mp *Mempool) CheckTxWithInfo(tx types.Tx,
	callback func(*abci.Response),
	txInfo mempool.TxInfo) error {
	mp.proxyMtx.Lock()
	defer mp.proxyMtx.Unlock()

	var (
		memSize  = mp.Size()
		txsBytes = mp.TxsBytes()
		txSize   = len(tx)
	)

	// Check whether the pool has sufficient capacity
	// to accommodate this new transaction
	if memSize >= mp.config.Mempool.Size ||
		int64(txSize)+txsBytes > mp.config.Mempool.MaxTxsSize {
		return fmt.Errorf(
			"mempool is full: number of txs %d (max: %d), total txs bytes %d (max: %d)",
			memSize, mp.config.Mempool.Size, txsBytes, mp.config.Mempool.MaxTxsSize)
	}

	// The size of the corresponding amino-encoded TxMessage
	// can't be larger than the maxMsgSize, otherwise we can't
	// relay it to peers.
	if txSize > mp.config.Mempool.MaxTxSize {
		return fmt.Errorf("Tx too large. Max size is %d, but got %d",
			mp.config.Mempool.MaxTxSize, txSize)
	}

	// NOTE: proxyAppConn may error if tx buffer is full
	if err := mp.proxyAppConn.Error(); err != nil {
		return err
	}

	// Pass the transaction to the proxy app so checks are performed
	reqRes := mp.proxyAppConn.CheckTxAsync(abci.RequestCheckTx{Tx: tx})
	reqRes.SetCallback(mp.onTxCheckFinished(tx, txInfo.SenderID, callback))

	return nil
}

// onTxCheckFinished returns a callback function that is called
// when the abci app checks the transaction.
// The argument externalCb is a callback that is called after onTxCheckFinished
// has finished its operations. It used by external callers to initiate
// other operations that need to be executed after onTxCheckFinished finishes.
func (mp *Mempool) onTxCheckFinished(tx []byte, peerID uint16,
	externalCb func(*abci.Response)) func(res *abci.Response) {
	return func(res *abci.Response) {

		// Add the transaction to the pool
		mp.addTx(tx, res)

		// passed in by the caller of CheckTx, eg. the RPC
		if externalCb != nil {
			externalCb(res)
		}
	}
}

// addTx adds a transaction to the pool
func (mp *Mempool) addTx(bs []byte, res *abci.Response) {

	switch r := res.Value.(type) {
	case *abci.Response_CheckTx:

		// At this point, the transaction the ABCI app's check
		if r.CheckTx.Code != abci.CodeTypeOK {
			mp.log.Info("Rejected bad transaction")
			return
		}

		tx, _ := t.NewTxFromBytes(bs)
		err := mp.pool.Put(tx)
		if err != nil {
			r.CheckTx.Code = t.ErrCodeTxPoolReject
			r.CheckTx.Log = err.Error()
			return
		}

		mp.log.Info("Added a new transaction to the pool", "Hash", tx.GetHash())
		mp.notifyTxsAvailable()
	}
}

// notifiedTxsAvailable signals through a channel that a transaction is available
func (mp *Mempool) notifyTxsAvailable() {
	if mp.Size() == 0 {
		panic("notified txs available but mempool is empty!")
	}
	if mp.txsAvailable != nil && !mp.notifiedTxsAvailable {
		// channel cap is 1, so this will send once
		mp.notifiedTxsAvailable = true
		select {
		case mp.txsAvailable <- struct{}{}:
		default:
		}
	}
}

// ReapMaxBytesMaxGas reaps transactions from the mempool up to maxBytes
// bytes total. If both maxBytes are negative, there is no cap on the
// size of all returned transactions.
// NOTE: maxGas is ignored since this mempool does not apply the concept of gas
func (mp *Mempool) ReapMaxBytesMaxGas(maxBytes, maxGas int64) types.Txs {
	mp.proxyMtx.Lock()
	defer mp.proxyMtx.Unlock()

	var totalBytes int64
	txs := make([]types.Tx, 0, mp.pool.Size())
	numValTicketTxReaped := 0
	ignoredTx := []t.Tx{}

	// Get an epoch secret and add it as the first reaped tx.
	// Exit immediately when we are unable to obtain drand random value.
	// This is necessary to prevent the validator for proposing a
	// epoch end block that has no epoch secret.
	if mp.epochSecretGetter != nil {
		epochSecretTx, err := mp.epochSecretGetter()
		if err != nil {
			mp.log.Fatal(err.Error())
		} else if epochSecretTx != nil {
			txBs := epochSecretTx.Bytes()
			txs = append(txs, txBs)
			totalBytes += int64(len(txBs))
			mp.log.Debug("Fetched and appended next epoch secret to reaped txs")
		}
	}

	// Collect transactions from the top
	// of the pool up to the given maxBytes.
	for {

		// Fetch a transaction. Exit if nil is returned.
		memTx := mp.pool.Head()
		if memTx == nil {
			break
		}

		// if tx is a validator ticket and we already reaped n
		// validator tickets, we cache and ignore it. We will
		// flush them back to the pool after reaping.
		if memTx.GetType() == t.TxTypeGetTicket &&
			numValTicketTxReaped == params.MaxValTicketsPerBlock {
			ignoredTx = append(ignoredTx, memTx)
			continue
		}

		// Check total size requirement
		txBs := memTx.Bytes()
		txSize := len(txBs)
		if maxBytes > -1 && totalBytes+int64(txSize) > maxBytes {
			return txs
		}
		totalBytes += int64(txSize)

		txs = append(txs, txBs)

		// Increment num validator tickets seen
		if memTx.GetType() == t.TxTypeGetTicket {
			numValTicketTxReaped++
		}
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
func (mp *Mempool) ReapMaxTxs(max int) types.Txs {
	// not implemented
	return nil
}

// Lock locks the mempool. The consensus must be able to hold lock to safely update.
func (mp *Mempool) Lock() {
	mp.proxyMtx.Lock()
}

// Unlock unlocks the mempool.
func (mp *Mempool) Unlock() {
	mp.proxyMtx.Unlock()
}

// Update informs the mempool that the given txs were committed and can be discarded.
// NOTE: this should be called *after* block is committed by consensus.
// NOTE: unsafe; Lock/Unlock must be managed by caller
func (mp *Mempool) Update(blockHeight int64, txs types.Txs,
	deliverTxResponses []*abci.ResponseDeliverTx,
	newPreFn mempool.PreCheckFunc,
	newPostFn mempool.PostCheckFunc) error {

	mp.notifiedTxsAvailable = false

	// Remove the transactions
	for _, txBs := range txs {
		tx, _ := t.NewTxFromBytes(txBs)
		mp.pool.Remove(tx)
	}

	// Notify that there are transactions still in the pool
	// if it is not empty
	if mp.Size() > 0 {
		mp.notifyTxsAvailable()
	}

	return nil
}

// FlushAppConn flushes the mempool connection to ensure async reqResCb calls are
// done. E.g. from CheckTx.
func (mp *Mempool) FlushAppConn() error {
	return mp.proxyAppConn.FlushSync()
}

// Flush removes all transactions from the mempool and cache
func (mp *Mempool) Flush() {
}

// TxsAvailable returns a channel which fires once for every height,
// and only when transactions are available in the mempool.
// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
func (mp *Mempool) TxsAvailable() <-chan struct{} {
	return mp.txsAvailable
}

// EnableTxsAvailable initializes the TxsAvailable channel, ensuring it will
// trigger once every height when transactions are available.
func (mp *Mempool) EnableTxsAvailable() {
	mp.txsAvailable = make(chan struct{}, 1)
}

// Size returns the number of transactions in the mempool.
func (mp *Mempool) Size() int {
	return int(mp.pool.Size())
}

// TxsBytes returns the total size of all txs in the mempool.
func (mp *Mempool) TxsBytes() int64 {
	return mp.pool.ActualSize()
}

func (mp *Mempool) globalCb(req *abci.Request, res *abci.Response) {}

// InitWAL creates a directory for the WAL file and opens a file itself.
func (mp *Mempool) InitWAL() {
}

// CloseWAL closes and discards the underlying WAL file.
// Any further writes will not be relayed to disk.
func (mp *Mempool) CloseWAL() {
	return
}
