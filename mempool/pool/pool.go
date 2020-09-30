package pool

import (
	"sync"
	"time"

	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/olebedev/emitter"
)

// PushPool wraps the transaction container providing a pool
// that can be used to store and manage transactions awaiting
// inclusion into blocks.
type Pool struct {
	sync.RWMutex
	container *Container
	keeper    core.Keepers
}

// New creates a new instance of pool.
// cap is the max amount of transactions that can be maintained in the pool.
// keepers is the application data keeper provider.
// bus is the app's event emitter provider.
func New(cap int, keepers core.Keepers, bus *emitter.Emitter) *Pool {
	pool := &Pool{RWMutex: sync.RWMutex{}, keeper: keepers}
	pool.container = NewContainer(cap, bus, pool.getNonce)
	return pool
}

// getNonce returns the account nonce of a given address
func (tp *Pool) getNonce(address string) (uint64, error) {
	acct := tp.keeper.AccountKeeper().Get(identifier.Address(address))
	if acct.IsNil() {
		return 0, types.ErrAccountUnknown
	}
	return acct.Nonce.UInt64(), nil
}

// Remove removes one or many transactions
func (tp *Pool) Remove(txs ...types.BaseTx) {
	tp.Lock()
	defer tp.Unlock()
	tp.container.remove(txs...)
}

// Put adds a transaction.
//  - Returns true and nil of tx was added to the pool.
//  - Returns false and nil if tx was added to the cache.
//  - Emits EvtMempoolTxCommitted if tx was successfully
//
// CONTRACTS:
//  - No two transactions with same sender, nonce and fee rate is allowed.
//  - Transactions are always ordered by nonce (ASC) and fee rate (DESC).
func (tp *Pool) Put(tx types.BaseTx) (bool, error) {
	tp.Lock()
	defer tp.Unlock()
	addedToPool, err := tp.addTx(tx)
	if err != nil {
		return false, err
	}
	return addedToPool, nil
}

// addTx adds a transaction to the container.
// Not safe for concurrent use.
func (tp *Pool) addTx(tx types.BaseTx) (bool, error) {
	if tp.container.Has(tx) {
		return false, ErrTxAlreadyAdded
	}
	return tp.container.Add(tx)
}

// Has checks whether a transaction is in the pool
func (tp *Pool) Has(tx types.BaseTx) bool {
	return tp.container.Has(tx)
}

// HasByHash is like Has but accepts a hash
func (tp *Pool) HasByHash(hash string) bool {
	return tp.container.HasByHash(hash)
}

// Get iterates over the transactions and invokes iteratee for
// each transaction. The iteratee is invoked the transaction as the
// only argument. It immediately stops and returns the last retrieved
// transaction when the iteratee returns true.
func (tp *Pool) Find(iteratee func(tx types.BaseTx, feeRate util.String, timeAdded time.Time) bool) types.BaseTx {
	return tp.container.find(iteratee)
}

// ByteSize gets the total byte size of all transactions in the pool.
// Note: The fee field of the transaction is not calculated.
func (tp *Pool) ByteSize() int64 {
	return tp.container.ByteSize()
}

// Size gets the total number of transactions in the pool
func (tp *Pool) Size() int {
	return tp.container.Size()
}

// CacheSize returns the size of the cache
func (tp *Pool) CacheSize() int {
	return tp.container.CacheSize()
}

// GetFromCache gets a transaction from the cache.
// Blocks if cache channel is empty
func (tp *Pool) GetFromCache() types.BaseTx {
	return tp.container.cache.Get()
}

// Flush clears the container and caches
func (tp *Pool) Flush() {
	tp.container.Flush()
}

// GetByHash gets a transaction from the pool using its hash
func (tp *Pool) GetByHash(hash string) types.BaseTx {
	return tp.container.GetByHash(hash)
}

// Head returns transaction from the top of the pool.
func (tp *Pool) Head() types.BaseTx {
	return tp.container.First()
}
