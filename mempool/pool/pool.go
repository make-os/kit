package pool

import (
	"gitlab.com/makeos/mosdef/types/msgs"
	"sync"
	"time"

	"gitlab.com/makeos/mosdef/util"

	"gitlab.com/makeos/mosdef/params"
)

// Pool stores transactions.
type Pool struct {
	sync.RWMutex              // general mutex
	container    *TxContainer // transaction queue
}

// New creates a new instance of pool.
// Cap size is the max amount of transactions
// that can be maintained in the pool.
func New(cap int64) *Pool {
	tp := new(Pool)
	tp.container = newTxContainer(cap)
	return tp
}

// Remove removes transactions
func (tp *Pool) Remove(txs ...msgs.BaseTx) {
	tp.Lock()
	defer tp.Unlock()
	tp.container.Remove(txs...)
	tp.clean()
}

// Put adds a transaction.
// CONTRACT: No two transactions with same sender, nonce and fee rate is allowed.
// CONTRACT: Transactions are always ordered by nonce (ASC) and fee rate (DESC).
func (tp *Pool) Put(tx msgs.BaseTx) error {
	tp.Lock()
	defer tp.Unlock()

	if err := tp.addTx(tx); err != nil {
		return err
	}

	tp.clean()

	return nil
}

// isExpired checks whether a transaction has expired
func (tp *Pool) isExpired(tx msgs.BaseTx) bool {
	expTime := time.Unix(tx.GetTimestamp(), 0).UTC().AddDate(0, 0, params.TxTTL)
	return time.Now().UTC().After(expTime)
}

// clean removes old transactions
// FIXME: clean transactions that have spent x period in the pool as opposed
// to how long they have existed themselves.
func (tp *Pool) clean() {
	tp.container.Find(func(tx msgs.BaseTx) bool {
		if tp.isExpired(tx) {
			tp.container.remove(tx)
		}
		return false
	})
}

// addTx adds a transaction to the queue.
// (Not thread-safe)
func (tp *Pool) addTx(tx msgs.BaseTx) error {

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
func (tp *Pool) Has(tx msgs.BaseTx) bool {
	return tp.container.Has(tx)
}

// HasByHash is like Has but accepts a hash
func (tp *Pool) HasByHash(hash string) bool {
	return tp.container.HasByHash(hash)
}

// Find iterates over the transactions and invokes iteratee for
// each transaction. The iteratee is invoked the transaction as the
// only argument. It immediately stops and returns the last retrieved
// transaction when the iteratee returns true.
func (tp *Pool) Find(iteratee func(msgs.BaseTx) bool) msgs.BaseTx {
	return tp.container.Find(iteratee)
}

// ByteSize gets the total byte size of all transactions in the pool.
// Note: The fee field of the transaction is not calculated.
func (tp *Pool) ByteSize() int64 {
	return tp.container.ByteSize()
}

// ActualSize gets the total byte size of all transactions in the pool.
// All fields are calculated, unlike ByteSize.
func (tp *Pool) ActualSize() int64 {
	return tp.container.ActualSize()
}

// Size gets the total number of transactions
// in the pool
func (tp *Pool) Size() int64 {
	return tp.container.Size()
}

// GetByHash gets a transaction from the pool using its hash
func (tp *Pool) GetByHash(hash string) msgs.BaseTx {
	return tp.container.GetByHash(hash)
}

// GetByFrom fetches transactions where the sender
// or `from` field match the given address
func (tp *Pool) GetByFrom(address util.String) []msgs.BaseTx {
	var txs []msgs.BaseTx
	tp.container.Find(func(tx msgs.BaseTx) bool {
		if tx.GetFrom().Equal(address) {
			txs = append(txs, tx)
		}
		return false
	})
	return txs
}

// Head returns transaction from the top of the pool.
func (tp *Pool) Head() msgs.BaseTx {
	return tp.container.First()
}
