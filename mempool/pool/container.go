package pool

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	dll "github.com/emirpasic/gods/lists/doublylinkedlist"
	memtypes "github.com/make-os/kit/mempool/types"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/olebedev/emitter"
	"github.com/shopspring/decimal"
)

var (
	// ErrContainerFull is an error about a full container
	ErrContainerFull = fmt.Errorf("container is full")

	// ErrTxAlreadyAdded is an error about a transaction
	// that is in the container.
	ErrTxAlreadyAdded = fmt.Errorf("exact transaction already in the pool")

	// ErrSenderTxLimitReached is an error about a sender reaching the container's tx limit per sender
	ErrSenderTxLimitReached = fmt.Errorf("sender's pool transaction limit reached")

	// ErrFailedReplaceByFee means an attempt to replace by fee failed due to the replacement
	// tx having a lower/equal fee to the current
	ErrFailedReplaceByFee = fmt.Errorf("an existing transaction by " +
		"same sender and at same nonce exist in the mempool. To replace the " +
		"existing transaction, the new transaction fee must be higher")
)

// Container represents the internal container used by the container.
// It provides a Put operation with sorting by fee rate and nonce.
// The container is thread-safe.
type Container struct {
	lck              *sync.RWMutex
	container        *dll.List              // main transaction container (the container).
	cap              int                    // cap is the maximum amount of transactions allowed.
	noSorting        bool                   // indicates that sorting should be disabled.
	hashIndex        map[string]interface{} // indexes a transactions hash for quick existence lookup.
	byteSize         int64                  // the total transaction size of the container
	senderNonceIndex senderNonces           // indexes sending addresses to nonce of transactions signed by them.
	cache            *Cache                 // Transactions to be re-attempted
	getNonce         NonceGetterFunc        // Function for getting nonce of an account
	bus              *emitter.Emitter       // Event emitter
}

// NewContainer creates a new Container
func NewContainer(cap int, bus *emitter.Emitter, getNonce NonceGetterFunc) *Container {
	return &Container{
		lck:              &sync.RWMutex{},
		container:        dll.New(),
		cap:              cap,
		hashIndex:        make(map[string]interface{}),
		senderNonceIndex: make(map[identifier.Address]*nonceCollection),
		cache:            NewCache(),
		getNonce:         getNonce,
		byteSize:         0,
		bus:              bus,
	}
}

// NewTxContainerNoSort creates a new container
// with sorting turned off
func NewTxContainerNoSort(cap int, bus *emitter.Emitter, getNonce NonceGetterFunc) *Container {
	return &Container{
		lck:              &sync.RWMutex{},
		container:        dll.New(),
		cap:              cap,
		hashIndex:        make(map[string]interface{}),
		senderNonceIndex: make(map[identifier.Address]*nonceCollection),
		cache:            NewCache(),
		noSorting:        true,
		getNonce:         getNonce,
		byteSize:         0,
		bus:              bus,
	}
}

// Size returns the number of items in the container
func (c *Container) Size() int {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.container.Size()
}

// ByteSize gets the total byte size of
// all transactions in the container.
// Note: The size of fee field of transactions are not calculated.
func (c *Container) ByteSize() int64 {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.byteSize
}

// Full checks if the container's capacity has been reached
func (c *Container) Full() bool {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.Size() >= c.cap
}

// CacheSize returns the size of the cache
func (c *Container) CacheSize() int {
	return c.cache.Size()
}

// calcFeeRate calculates the fee rate of a transaction
func calcFeeRate(tx types.BaseTx) util.String {
	txSizeDec := decimal.NewFromBigInt(new(big.Int).SetInt64(tx.GetEcoSize()), 0)
	return util.String(tx.GetFee().Decimal().Div(txSizeDec).String())
}

// Add adds a transaction to the container.
//
// After addition:
//  - The pool is sorted (if sorting is enabled).
//  - EvtMempoolTxAdded is fired.
//  - Expired txs are removed from the container.
//
// Returns true and nil if tx was added to the container.
// Returns false and nil if tx was added to the cache.
// Returns error if there was a problem with the tx.
func (c *Container) Add(tx types.BaseTx) (bool, error) {
	c.lck.Lock()
	defer c.lck.Unlock()

	// Calculate the transaction's fee rate (tx fee / size)
	item := newItem(tx)
	item.FeeRate = calcFeeRate(tx)

	// Get the sender's nonce info. If not found create a new one
	sender := tx.GetFrom()
	senderNonceInfo := c.senderNonceIndex.get(sender)
	var ni *nonceInfo

	// If no existing transaction with a matching nonce, Add this tx nonce to
	// the nonce index and proceed to include the transaction
	if !senderNonceInfo.has(tx.GetNonce()) {
		senderNonceInfo.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash(), Fee: item.Tx.GetFee()})
		goto add
	}

	// However, reject a transaction if their is already a matching
	// nonce from same sender that has an equal or higher fee.
	// CONTRACT: To replace-by-fee, the new transaction must have a higher fee.
	ni = senderNonceInfo.get(tx.GetNonce())
	if ni.Fee.Decimal().GreaterThanOrEqual(item.Tx.GetFee().Decimal()) {
		return false, ErrFailedReplaceByFee
	}

	// At the point, the new transaction has a higher fee rate, therefore we
	// need to remove the existing transaction and replace with the new one
	// and also replace the nonce information
	c.removeByHash(ni.TxHash)
	senderNonceInfo.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash(), Fee: item.Tx.GetFee()})

add:

	// Check per-sender pool transaction limit
	if c.sizeByAddr(sender) == params.MempoolSenderTxLimit {
		return false, ErrSenderTxLimitReached
	}

	// Ensure cap has not been reached
	if c.container.Size() >= c.cap {
		return false, ErrContainerFull
	}

	// Get the current nonce of sender
	curSenderNonce, err := c.getNonce(tx.GetFrom().String())
	if err != nil {
		if err != types.ErrAccountUnknown {
			return false, err
		}
	}

	// Ensure the transaction nonce is not lower/equal than/to current account nonce.
	if curSenderNonce >= tx.GetNonce() {
		return false, fmt.Errorf("tx nonce cannot be less than or equal to current account nonce")
	}

	// When the tx nonce is not the next expected nonce
	// and the immediate nonce (n-1) of the tx is not in the container, cache the tx.
	if tx.GetNonce()-curSenderNonce > 1 {
		if !senderNonceInfo.has(tx.GetNonce() - 1) {
			if err := c.cache.Add(tx); err != nil {
				return false, err
			}
			return false, nil
		}
	}

	c.senderNonceIndex[sender] = senderNonceInfo
	c.container.Append(item)
	c.hashIndex[tx.GetHash().String()] = struct{}{}
	c.byteSize += tx.GetEcoSize()

	if !c.noSorting {
		c.Sort()
	}

	go c.maybeProcessCache()
	c.clean()
	c.bus.Emit(memtypes.EvtMempoolTxAdded, nil, tx)

	return true, nil
}

// maybeProcessCache attempts a add tx in the cache to the main pool
func (c *Container) maybeProcessCache() (added bool, err error) {
	for {
		// Get a tx from the cache
		tx := c.cache.Get()
		if tx == nil {
			return false, nil
		}

		// Attempt to add it to the container.
		added, err = c.Add(tx)
		if err != nil || !added {
			c.bus.Emit(memtypes.EvtMempoolTxRejected, err, tx)
			return false, err
		}

		// Emit EvtMempoolBroadcastTx to broadcast the tx
		c.bus.Emit(memtypes.EvtMempoolBroadcastTx, tx)
	}
}

// sizeByAddr returns the number of transactions signed by a given address.
// Not thread safe; Must be called with lock held.
func (c *Container) sizeByAddr(addr identifier.Address) int {
	var poolCount int
	ni, ok := c.senderNonceIndex[addr]
	if ok {
		poolCount = len(ni.nonces)
	}
	return c.cache.SizeByAddr(addr) + poolCount
}

// SizeByAddr returns the number of transactions signed by a given address
func (c *Container) SizeByAddr(addr identifier.Address) int {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.sizeByAddr(addr)
}

// Get returns a transaction at the given index
func (c *Container) Get(index int) *containerItem {
	c.lck.RLock()
	defer c.lck.RUnlock()
	res, ok := c.container.Get(index)
	if ok {
		return res.(*containerItem)
	}
	return nil
}

// Has checks whether a transaction is in the container
func (c *Container) Has(tx types.BaseTx) bool {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.hashIndex[tx.GetHash().String()] != nil
}

// HasByHash is like Has but accepts a transaction hash
func (c *Container) HasByHash(hash string) bool {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.hashIndex[hash] != nil
}

// First returns the transaction at the head of the container.
// Returns nil if container is empty
func (c *Container) First() types.BaseTx {

	if c.Size() <= 0 {
		return nil
	}

	c.lck.Lock()
	defer c.lck.Unlock()

	// Get a transaction from the list
	item, _ := c.container.Get(0)
	c.container.Remove(0)
	tx := item.(*containerItem).Tx

	// Delete the tx from caches
	delete(c.hashIndex, tx.GetHash().String())
	c.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())

	// Decrement counts
	c.byteSize -= tx.GetEcoSize()

	return tx
}

// Last returns the transaction at the back of the container.
// Returns nil if container is empty
func (c *Container) Last() types.BaseTx {

	if c.Size() <= 0 {
		return nil
	}

	c.lck.Lock()
	defer c.lck.Unlock()

	// Get a transaction from the list
	item, _ := c.container.Get(c.container.Size() - 1)
	c.container.Remove(c.container.Size() - 1)
	tx := item.(*containerItem).Tx

	// Delete the tx from caches
	delete(c.hashIndex, tx.GetHash().String())
	c.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())

	// Decrement counts
	c.byteSize -= tx.GetEcoSize()

	return tx
}

// Sort sorts the container
func (c *Container) Sort() {
	c.container.Sort(func(a, b interface{}) int {
		aItem := a.(*containerItem)
		bItem := b.(*containerItem)

		// When transaction a & b belong to same sender, sort by nonce in ascending order.
		if aItem.Tx.GetFrom() == bItem.Tx.GetFrom() {
			if aItem.Tx.GetNonce() < bItem.Tx.GetNonce() {
				return -1
			} else if aItem.Tx.GetNonce() > bItem.Tx.GetNonce() {
				return 1
			}
			return 0
		}

		// For non-sender matching transactions, sort by highest fee rate
		if aItem.FeeRate.Decimal().GreaterThan(bItem.FeeRate.Decimal()) {
			return -1
		} else if aItem.FeeRate.Decimal().LessThan(bItem.FeeRate.Decimal()) {
			return 1
		}

		return 0
	})
}

// Find calls the given function once for every transaction and
// returns the transaction for which the function returns true for.
// Not safe for concurrent use.
func (c *Container) find(iteratee func(types.BaseTx, util.String, time.Time) bool) types.BaseTx {
	_, res := c.container.Find(func(index int, value interface{}) bool {
		item := value.(*containerItem)
		return iteratee(item.Tx, item.FeeRate, item.TimeAdded)
	})
	if res != nil {
		return res.(*containerItem).Tx
	}
	return nil
}

// remove removes one or more transactions.
// Note: Not safe for concurrent calls
func (c *Container) remove(txsToDel ...types.BaseTx) {
	defer c.clean()
	c.container = c.container.Select(func(index int, value interface{}) bool {
		tx := value.(*containerItem).Tx
		for _, txToDel := range txsToDel {
			if !tx.GetHash().Equal(txToDel.GetHash()) {
				continue
			}
			delete(c.hashIndex, tx.GetHash().String())
			c.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())
			c.byteSize -= tx.GetEcoSize()
			return false
		}
		return true
	})
}

// removeByHash removes transactions by hash
// Note: Not safe for concurrent calls
func (c *Container) removeByHash(txsHash ...util.HexBytes) {
	c.container.Each(func(index int, value interface{}) {
		tx := value.(*containerItem).Tx
		for _, hash := range txsHash {
			if !tx.GetHash().Equal(hash) {
				continue
			}
			c.container.Remove(index)
			delete(c.hashIndex, tx.GetHash().String())
			c.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())
			c.byteSize -= tx.GetEcoSize()
		}
	})
}

// clean removes old transactions.
// Note: not thread safe.
func (c *Container) clean() {
	c.find(func(tx types.BaseTx, feeRate util.String, timeAdded time.Time) bool {
		expTime := timeAdded.Add(params.MempoolTxTTL)
		if time.Now().After(expTime) {
			c.remove(tx)
		}
		return false
	})
}

// Remove removes a transaction
func (c *Container) Remove(txs ...types.BaseTx) {
	c.lck.Lock()
	defer c.lck.Unlock()
	c.remove(txs...)
}

// GetByHash get a transaction by its hash from the container
func (c *Container) GetByHash(hash string) types.BaseTx {
	c.lck.RLock()
	defer c.lck.RUnlock()
	return c.find(func(tx types.BaseTx, _ util.String, _ time.Time) bool {
		return tx.GetHash().String() == hash
	})
}

// GetFeeRateByHash get a transaction's fee rate by its hash
func (c *Container) GetFeeRateByHash(hash string) util.String {
	c.lck.RLock()
	defer c.lck.RUnlock()
	var res util.String
	item := c.find(func(tx types.BaseTx, feeRate util.String, timeAdded time.Time) bool {
		res = feeRate
		return tx.GetHash().String() == hash
	})
	if item != nil {
		return res
	}
	return ""
}

// Flush clears the container and caches
func (c *Container) Flush() {
	c.lck.Lock()
	defer c.lck.Unlock()
	c.container.Clear()
	c.hashIndex = make(map[string]interface{})
	c.byteSize = 0
	c.senderNonceIndex = senderNonces{}
}
