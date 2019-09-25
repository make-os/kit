package reactors

import (
	"time"

	"github.com/makeos/mosdef/node/tmrpc"

	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/txpool"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/util/logger"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

const (
	// TxReactorChannel is a channel for processing transaction messages
	TxReactorChannel = byte(0x64)
)

// PoolSizeInfo describes the transaction byte size an count of the tx pool
type PoolSizeInfo struct {
	TotalTxSize int64
	TxCount     int64
}

// TxReactor processes incoming transactions from peers and other local services.
type TxReactor struct {
	p2p.BaseReactor
	log   logger.Logger
	name  string
	pool  *txpool.TxPool
	logic types.Logic
	tmrpc *tmrpc.TMRPC
}

// NewTxReactor creates an instance of TxReactor
func NewTxReactor(
	name string,
	poolCap int64,
	logic types.Logic,
	tmrpc *tmrpc.TMRPC,
	log logger.Logger) *TxReactor {
	tr := &TxReactor{
		name:  name,
		logic: logic,
		log:   log.Module("TxReator"),
		pool:  txpool.New(poolCap),
		tmrpc: tmrpc,
	}
	tr.BaseReactor = *p2p.NewBaseReactor(name, tr)
	go tr.reapToMempool()
	return tr
}

// GetName returns the name of the reactor
func (r *TxReactor) GetName() string {
	return r.name
}

// GetChannels implements Reactor
func (r *TxReactor) GetChannels() []*conn.ChannelDescriptor {
	return []*conn.ChannelDescriptor{
		{ID: TxReactorChannel, Priority: 1, SendQueueCapacity: 10},
	}
}

// OnStart is called when the reactor is started
func (r *TxReactor) OnStart() error {
	return nil
}

// AddPeer implements Reactor
func (r *TxReactor) AddPeer(p p2p.Peer) {}

// RemovePeer implements Reactor
func (r *TxReactor) RemovePeer(p p2p.Peer, reason interface{}) {}

// Receive implements Reactor. Receives a transasction that must
// be added into the pool.
// CONTRACT: Transaction is validated
func (r *TxReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {

	tx, err := types.NewTxFromBytes(msgBytes)
	if err != nil {
		r.log.Error("Failed to decode received transaction")
		return
	}

	// Validate the transaction
	if err = validators.ValidateTx(tx, -1, r.logic); err != nil {
		r.log.Error("Received transaction is not valid", "Err", err.Error())
		return
	}

	// Add the transaction to the pool
	if err = r.pool.Put(tx); err != nil {
		r.log.Error("Failed to add transaction into the pool", "Err", err.Error())
		return
	}

	r.log.Debug("Received and added a new transaction to the pool", "Hash", tx.GetHash())
}

// AddTx adds a transaction to the tx pool and broadcasts it.
// CONTRACT: Caller must validate the transaction before call.
func (r *TxReactor) AddTx(tx types.Tx) (hash util.Hash, err error) {

	// Add the transaction to put
	if err := r.pool.Put(tx); err != nil {
		return util.EmptyHash, err
	}

	// Broadcast the transaction
	r.Switch.Broadcast(TxReactorChannel, tx.Bytes())

	return tx.GetHash(), nil
}

// GetPoolSize returns the size information of the pool
func (r *TxReactor) GetPoolSize() *PoolSizeInfo {
	return &PoolSizeInfo{
		TotalTxSize: r.pool.ByteSize(),
		TxCount:     r.pool.Size(),
	}
}

// GetTop returns the top n transactions in the pool.
// It will return all transactions if n is zero or negative.
func (r *TxReactor) GetTop(n int) []types.Tx {
	var txs []types.Tx
	r.pool.Find(func(tx types.Tx) bool {
		txs = append(txs, tx)
		if n > 0 && len(txs) == n {
			return true
		}
		return false
	})
	return txs
}

// reapToMempool starts a routine that pulls transactions from the
// transaction pool and sends them to tendermint's mempool. It puts
// back the transaction to the transaction pool if it fails to
// send them to the mempool.
func (r *TxReactor) reapToMempool() {
	t := time.NewTicker(1 * time.Second)
	for range t.C {
		tx := r.pool.Head()
		if tx == nil {
			continue
		}
		_, err := r.tmrpc.SendTx(tx.Bytes())
		if err != nil {
			r.log.Error("Failed to push tx to tendermint mempool. Pushing back to txpool",
				"Err", err)
			r.pool.Put(tx)
		}
	}
}
