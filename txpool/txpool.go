package txpool

import (
	"sync"
	"time"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
)

// TxPool stores transactions.
type TxPool struct {
	sync.RWMutex              // general mutex
	container    *TxContainer // transaction queue
}

// New creates a new instance of TxPool.
// Cap size is the max amount of transactions
// that can be maintained in the pool.
func New(cap int64) *TxPool {
	tp := new(TxPool)
	tp.container = newTxContainer(cap)
	return tp
}

// Remove removes transactions
func (tp *TxPool) Remove(txs ...types.Tx) {
	tp.Lock()
	defer tp.Unlock()
	tp.container.Remove(txs...)
	tp.clean()
}

// Put adds a transaction.
// CONTRACT: No two transactions with same sender, nonce and fee rate is allowed.
// CONTRACT: Transactions are always ordered by nonce (ASC) and fee rate (DESC).
func (tp *TxPool) Put(tx types.Tx) error {
	tp.Lock()
	defer tp.Unlock()

	if err := tp.addTx(tx); err != nil {
		return err
	}

	tp.clean()

	return nil
}

// isExpired checks whether a transaction has expired
func (tp *TxPool) isExpired(tx types.Tx) bool {
	expTime := time.Unix(tx.GetTimestamp(), 0).UTC().AddDate(0, 0, params.TxTTL)
	return time.Now().UTC().After(expTime)
}

// clean removes old transactions
// FIXME: clean transactions that have spent x period in the pool as opposed
// to how long they have existed themselves.
func (tp *TxPool) clean() {
	tp.container.Find(func(tx types.Tx) bool {
		if tp.isExpired(tx) {
			tp.container.remove(tx)
		}
		return false
	})
}

// addTx adds a transaction to the queue.
// (Not thread-safe)
func (tp *TxPool) addTx(tx types.Tx) error {

	// Ensure the transaction does not already 
	// exist in the queue
	if tp.container.Has(tx) {
		return ErrTxAlreadyAdded
	}

	// Append the the transaction to the the queue.
	if err := tp.container.add(tx); err != nil {
		return err
	}

	return nil
}

// Has checks whether a transaction is in the pool
func (tp *TxPool) Has(tx types.Tx) bool {
	return tp.container.Has(tx)
}

// HasByHash is like Has but accepts a hash
func (tp *TxPool) HasByHash(hash string) bool {
	return tp.container.HasByHash(hash)
}

// Find iterates over the transactions and invokes iteratee for
// each transaction. The iteratee is invoked the transaction as the
// only argument. It immediately stops and returns the last retrieved
// transaction when the iteratee returns true.
func (tp *TxPool) Find(iteratee func(types.Tx) bool) types.Tx {
	return tp.container.Find(iteratee)
}

// ByteSize gets the total byte size of
// all transactions in the pool
func (tp *TxPool) ByteSize() int64 {
	return tp.container.ByteSize()
}

// Size gets the total number of transactions
// in the pool
func (tp *TxPool) Size() int64 {
	return tp.container.Size()
}

// GetByHash gets a transaction from the pool using its hash
func (tp *TxPool) GetByHash(hash string) types.Tx {
	return tp.container.GetByHash(hash)
}

// GetByFrom fetches transactions where the sender
// or `from` field match the given address
func (tp *TxPool) GetByFrom(address util.String) []types.Tx {
	var txs []types.Tx
	tp.container.Find(func(tx types.Tx) bool {
		if tx.GetFrom().Equal(address) {
			txs = append(txs, tx)
		}
		return false
	})
	return txs
}

// Head returns transaction from the top of the pool.
func (tp *TxPool) Head() types.Tx {
	return tp.container.First()
}
