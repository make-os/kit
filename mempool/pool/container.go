package pool

import (
	"fmt"
	"math/big"
	"sync"

	dll "github.com/emirpasic/gods/lists/doublylinkedlist"
	memtypes "github.com/make-os/lobe/mempool/types"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/olebedev/emitter"
	"github.com/shopspring/decimal"
)

var (
	// ErrContainerFull is an error about a full container
	ErrContainerFull = fmt.Errorf("container is full")

	// ErrTxAlreadyAdded is an error about a transaction
	// that is in the pool.
	ErrTxAlreadyAdded = fmt.Errorf("exact transaction already in the pool")

	// ErrSenderTxLimitReached is an error about a sender reaching the pool's tx limit per sender
	ErrSenderTxLimitReached = fmt.Errorf("sender's pool transaction limit reached")

	// ErrFailedReplaceByFee means an attempt to replace by fee failed due to the replacement
	// tx having a lower/equal fee to the current
	ErrFailedReplaceByFee = fmt.Errorf("an existing transaction by " +
		"same sender and at same nonce exist in the mempool. To replace the " +
		"existing transaction, the new transaction fee must be higher")
)

// TxContainer represents the internal container used by the pool.
// It provides a Put operation with sorting by fee rate and nonce.
// The container is thread-safe.
type TxContainer struct {
	lck              *sync.RWMutex
	container        *dll.List              // main transaction container (the pool).
	Cap              int                    // cap is the maximum amount of transactions allowed.
	noSorting        bool                   // indicates that sorting should be disabled.
	hashIndex        map[string]interface{} // indexes a transactions hash for quick existence lookup.
	byteSize         int64                  // the total transaction size of the container
	senderNonceIndex senderNonces           // indexes sending addresses to nonce of transactions signed by them.
	cache            *Cache                 // Transactions to be re-attempted
	getNonce         NonceGetterFunc        // Function for getting nonce of an account
	bus              *emitter.Emitter       // Event emitter
}

// NewTxContainer creates a new container
func NewTxContainer(cap int, bus *emitter.Emitter, getNonce NonceGetterFunc) *TxContainer {
	return &TxContainer{
		lck:              &sync.RWMutex{},
		container:        dll.New(),
		Cap:              cap,
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
func NewTxContainerNoSort(cap int, bus *emitter.Emitter, getNonce NonceGetterFunc) *TxContainer {
	return &TxContainer{
		lck:              &sync.RWMutex{},
		container:        dll.New(),
		Cap:              cap,
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
func (q *TxContainer) Size() int {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.container.Size()
}

// ByteSize gets the total byte size of
// all transactions in the container.
// Note: The size of fee field of transactions are not calculated.
func (q *TxContainer) ByteSize() int64 {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.byteSize
}

// Full checks if the container's capacity has been reached
func (q *TxContainer) Full() bool {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.Size() >= q.Cap
}

// CacheSize returns the size of the cache
func (q *TxContainer) CacheSize() int {
	return q.cache.Size()
}

// calcFeeRate calculates the fee rate of a transaction
func calcFeeRate(tx types.BaseTx) util.String {
	txSizeDec := decimal.NewFromBigInt(new(big.Int).SetInt64(tx.GetEcoSize()), 0)
	return util.String(tx.GetFee().Decimal().Div(txSizeDec).String())
}

// Add adds a transaction to the end of the container.
// Returns false if container capacity has been reached.
// It computes the fee rate and sorts the transactions
// after addition.
func (q *TxContainer) Add(tx types.BaseTx) (bool, error) {
	q.lck.Lock()
	defer q.lck.Unlock()

	// Calculate the transaction's fee rate (tx fee / size)
	item := newItem(tx)
	item.FeeRate = calcFeeRate(tx)

	// Get the sender's nonce info. If not found create a new one
	sender := tx.GetFrom()
	senderNonceInfo := q.senderNonceIndex.get(sender)
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
	q.removeByHash(ni.TxHash)
	senderNonceInfo.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash(), Fee: item.Tx.GetFee()})

add:

	// Check per-sender pool transaction limit
	if q.sizeByAddr(sender) == params.MempoolSenderTxLimit {
		return false, ErrSenderTxLimitReached
	}

	// Ensure cap has not been reached
	if q.container.Size() >= q.Cap {
		return false, ErrContainerFull
	}

	// Get the current nonce of sender
	curSenderNonce, err := q.getNonce(tx.GetFrom().String())
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
	// and the immediate nonce (n-1) of the tx is not in the pool, cache the tx.
	if tx.GetNonce()-curSenderNonce > 1 {
		if !senderNonceInfo.has(tx.GetNonce() - 1) {
			if !q.cache.Add(tx) {
				return false, fmt.Errorf("cache already contains a transaction with matching sender and nonce")
			}
			return false, nil
		}
	}

	q.senderNonceIndex[sender] = senderNonceInfo
	q.container.Append(item)
	q.hashIndex[tx.GetHash().String()] = struct{}{}
	q.byteSize += tx.GetEcoSize()

	if !q.noSorting {
		q.Sort()
	}

	q.bus.Emit(memtypes.EvtMempoolTxAdded, nil, tx)

	return true, nil
}

// sizeByAddr returns the number of transactions signed by a given address.
// Not thread safe; Must be called with lock held.
func (q *TxContainer) sizeByAddr(addr identifier.Address) int {
	var poolCount int
	ni, ok := q.senderNonceIndex[addr]
	if ok {
		poolCount = len(ni.nonces)
	}
	return q.cache.SizeByAddr(addr) + poolCount
}

// SizeByAddr returns the number of transactions signed by a given address
func (q *TxContainer) SizeByAddr(addr identifier.Address) int {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.sizeByAddr(addr)
}

// Get returns a transaction at the given index
func (q *TxContainer) Get(index int) *containerItem {
	q.lck.RLock()
	defer q.lck.RUnlock()
	res, ok := q.container.Get(index)
	if ok {
		return res.(*containerItem)
	}
	return nil
}

// Has checks whether a transaction is in the container
func (q *TxContainer) Has(tx types.BaseTx) bool {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.hashIndex[tx.GetHash().String()] != nil
}

// HasByHash is like Has but accepts a transaction hash
func (q *TxContainer) HasByHash(hash string) bool {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.hashIndex[hash] != nil
}

// First returns the transaction at the head of the container.
// Returns nil if container is empty
func (q *TxContainer) First() types.BaseTx {

	if q.Size() <= 0 {
		return nil
	}

	q.lck.Lock()
	defer q.lck.Unlock()

	// Get a transaction from the list
	item, _ := q.container.Get(0)
	q.container.Remove(0)
	tx := item.(*containerItem).Tx

	// Delete the tx from caches
	delete(q.hashIndex, tx.GetHash().String())
	q.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())

	// Decrement counts
	q.byteSize -= tx.GetEcoSize()

	return tx
}

// Last returns the transaction at the back of the container.
// Returns nil if container is empty
func (q *TxContainer) Last() types.BaseTx {

	if q.Size() <= 0 {
		return nil
	}

	q.lck.Lock()
	defer q.lck.Unlock()

	// Get a transaction from the list
	item, _ := q.container.Get(q.container.Size() - 1)
	q.container.Remove(q.container.Size() - 1)
	tx := item.(*containerItem).Tx

	// Delete the tx from caches
	delete(q.hashIndex, tx.GetHash().String())
	q.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())

	// Decrement counts
	q.byteSize -= tx.GetEcoSize()

	return tx
}

// Sort sorts the container
func (q *TxContainer) Sort() {
	q.container.Sort(func(a, b interface{}) int {
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
func (q *TxContainer) find(iteratee func(types.BaseTx, util.String) bool) types.BaseTx {
	_, res := q.container.Find(func(index int, value interface{}) bool {
		return iteratee(value.(*containerItem).Tx, value.(*containerItem).FeeRate)
	})
	if res != nil {
		return res.(*containerItem).Tx
	}
	return nil
}

// remove removes one or more transactions.
// Note: Not safe for concurrent calls
func (q *TxContainer) remove(txsToDel ...types.BaseTx) {
	q.container = q.container.Select(func(index int, value interface{}) bool {
		tx := value.(*containerItem).Tx
		for _, txToDel := range txsToDel {
			if !tx.GetHash().Equal(txToDel.GetHash()) {
				continue
			}
			delete(q.hashIndex, tx.GetHash().String())
			q.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())
			q.byteSize -= tx.GetEcoSize()
			return false
		}
		return true
	})
}

// removeByHash removes transactions by hash
// Note: Not safe for concurrent calls
func (q *TxContainer) removeByHash(txsHash ...util.HexBytes) {
	q.container.Each(func(index int, value interface{}) {
		tx := value.(*containerItem).Tx
		for _, hash := range txsHash {
			if !tx.GetHash().Equal(hash) {
				continue
			}
			q.container.Remove(index)
			delete(q.hashIndex, tx.GetHash().String())
			q.senderNonceIndex.remove(tx.GetFrom(), tx.GetNonce())
			q.byteSize -= tx.GetEcoSize()
		}
	})
}

// Remove removes a transaction
func (q *TxContainer) Remove(txs ...types.BaseTx) {
	q.lck.Lock()
	defer q.lck.Unlock()
	q.remove(txs...)
}

// GetByHash get a transaction by its hash from the pool
func (q *TxContainer) GetByHash(hash string) types.BaseTx {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.find(func(tx types.BaseTx, feeRate util.String) bool {
		return tx.GetHash().String() == hash
	})
}

// GetFeeRateByHash get a transaction's fee rate by its hash
func (q *TxContainer) GetFeeRateByHash(hash string) util.String {
	q.lck.RLock()
	defer q.lck.RUnlock()
	var res util.String
	item := q.find(func(tx types.BaseTx, feeRate util.String) bool {
		res = feeRate
		return tx.GetHash().String() == hash
	})
	if item != nil {
		return res
	}
	return ""
}

// Flush clears the container and caches
func (q *TxContainer) Flush() {
	q.lck.Lock()
	defer q.lck.Unlock()
	q.container.Clear()
	q.hashIndex = make(map[string]interface{})
	q.byteSize = 0
	q.senderNonceIndex = senderNonces{}
}
