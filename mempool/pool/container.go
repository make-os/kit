package pool

import (
	"fmt"
	"math/big"
	"sync"

	dll "github.com/emirpasic/gods/lists/doublylinkedlist"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/util"
	"gitlab.com/makeos/lobe/util/identifier"
)

var (
	// ErrContainerFull is an error about a full container
	ErrContainerFull = fmt.Errorf("container is full")

	// ErrTxAlreadyAdded is an error about a transaction
	// that is in the pool.
	ErrTxAlreadyAdded = fmt.Errorf("exact transaction already in the pool")

	// ErrFailedReplaceByFee means an attempt to replace by fee failed due to the replacement
	// tx having a lower/equal fee to the current
	ErrFailedReplaceByFee = fmt.Errorf("an existing transaction by " +
		"same sender and at same nonce exist in the mempool. To replace the " +
		"existing transaction, the new transaction fee must be higher")
)

// containerItem represents the a container item.
// It wraps a transaction and other important properties.
type containerItem struct {
	Tx      types.BaseTx
	FeeRate util.String
}

// newItem creates an instance of ContainerItem
func newItem(tx types.BaseTx) *containerItem {
	item := &containerItem{Tx: tx}
	return item
}

// nonceInfo stores information about a transaction
// that is associated with a specific nonce
type nonceInfo struct {
	TxHash util.HexBytes
	Fee    util.String
}

// nonceCollection maps nonces with transaction information
type nonceCollection struct {
	nonces map[uint64]*nonceInfo
}

// defaultNonceCollection returns a base nonceCollection instance
func defaultNonceCollection() *nonceCollection {
	return &nonceCollection{nonces: map[uint64]*nonceInfo{}}
}

// has checks whether a nonce exists in the collection
func (c *nonceCollection) has(nonce uint64) bool {
	if _, ok := c.nonces[nonce]; ok {
		return true
	}
	return false
}

// get returns a nonce information.
// Returns nil if no result is found for the given nonce.
func (c *nonceCollection) get(nonce uint64) *nonceInfo {
	if !c.has(nonce) {
		return nil
	}
	return c.nonces[nonce]
}

// Add adds a nonce information. If a matching nonce
// already exist, it is replaced with the new nonce info.
func (c *nonceCollection) add(nonce uint64, ni *nonceInfo) {
	c.nonces[nonce] = ni
}

// remove removes a nonce information.
func (c *nonceCollection) remove(nonce uint64) {
	delete(c.nonces, nonce)
}

type senderNonces map[identifier.Address]*nonceCollection

// remove removes a nonce associated with a sender address.
// The entire map entry for the sender is removed if no other
// nonce exist after the operation
func (sn *senderNonces) remove(senderAddr identifier.Address, nonce uint64) {
	nc, ok := (*sn)[senderAddr]
	if !ok {
		return
	}
	nc.remove(nonce)
	if len(nc.nonces) == 0 {
		delete(*sn, senderAddr)
	}
}

// len returns the length of the collection
func (sn *senderNonces) len() int {
	return len(*sn)
}

// TxContainer represents the internal container
// used by pool. It provides a Put operation
// with automatic sorting by fee rate and nonce.
// The container is thread-safe.
type TxContainer struct {
	lck        *sync.RWMutex
	container  *dll.List              // main transaction container (the pool)
	Cap        int                    // cap is the amount of transactions in the
	noSorting  bool                   // noSorting indicates that sorting is enabled/disabled
	hashIndex  map[string]interface{} // hashIndex caches tx hashes for quick existence lookup
	byteSize   int64                  // byteSize is the total txs size of the container (excluding fee field)
	actualSize int64                  // actualSize is the total tx size of the container (includes all fields)

	// senderNonceIndex maps sending addresses to nonces of transaction.
	// We use this to find transactions from same sender with matching nonce for
	// implementing a replay-by-fee mechanism.
	senderNonceIndex senderNonces
}

// NewTxContainer creates a new container
func NewTxContainer(cap int) *TxContainer {
	q := new(TxContainer)
	q.container = dll.New()
	q.Cap = cap
	q.lck = &sync.RWMutex{}
	q.hashIndex = map[string]interface{}{}
	q.senderNonceIndex = map[identifier.Address]*nonceCollection{}
	return q
}

// NewTxContainerNoSort creates a new container
// with sorting turned off
func NewTxContainerNoSort(cap int) *TxContainer {
	q := new(TxContainer)
	q.container = dll.New()
	q.Cap = cap
	q.lck = &sync.RWMutex{}
	q.hashIndex = map[string]interface{}{}
	q.noSorting = true
	q.senderNonceIndex = map[identifier.Address]*nonceCollection{}
	return q
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

// ActualSize is like ByteSize but its result includes all fields
func (q *TxContainer) ActualSize() int64 {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.actualSize
}

// Full checks if the container's capacity has been reached
func (q *TxContainer) Full() bool {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.Size() >= q.Cap
}

// noSort checks whether sorting is disabled
func (q *TxContainer) noSort() bool {
	q.lck.RLock()
	defer q.lck.RUnlock()
	return q.noSorting
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
func (q *TxContainer) Add(tx types.BaseTx) error {

	if q.Full() {
		return ErrContainerFull
	}

	q.lck.Lock()

	// Calculate the transaction's fee rate (tx fee / size)
	item := newItem(tx)
	item.FeeRate = calcFeeRate(tx)

	// Get the sender's nonce info. If not found create a new one
	sender := tx.GetFrom()
	senderNonceInfo, ok := q.senderNonceIndex[sender]
	if !ok {
		senderNonceInfo = defaultNonceCollection()
	}

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
		q.lck.Unlock()
		return ErrFailedReplaceByFee
	}

	// At the point, the new transaction has a higher fee rate, therefore we
	// need to remove the existing transaction and replace with the new one
	// and also Add also replace the nonce information
	q.removeByHash(ni.TxHash)
	senderNonceInfo.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash(), Fee: item.Tx.GetFee()})

add:

	q.senderNonceIndex[sender] = senderNonceInfo
	q.container.Append(item)
	q.hashIndex[tx.GetHash().String()] = struct{}{}
	q.byteSize += tx.GetEcoSize()
	q.actualSize += int64(len(tx.Bytes()))

	q.lck.Unlock()

	if !q.noSort() {
		q.Sort()
	}

	return nil
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
	q.actualSize -= int64(len(tx.Bytes()))

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
	q.actualSize -= int64(len(tx.Bytes()))

	return tx
}

// Sort sorts the container
func (q *TxContainer) Sort() {
	q.lck.Lock()
	defer q.lck.Unlock()
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
func (q *TxContainer) Find(iteratee func(types.BaseTx, util.String) bool) types.BaseTx {
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
			q.actualSize -= int64(len(tx.Bytes()))
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
			q.actualSize -= int64(len(tx.Bytes()))
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
	q.lck.Lock()
	defer q.lck.Unlock()
	return q.Find(func(tx types.BaseTx, feeRate util.String) bool {
		return tx.GetHash().String() == hash
	})
}

// GetFeeRateByHash get a transaction's fee rate by its hash
func (q *TxContainer) GetFeeRateByHash(hash string) util.String {
	q.lck.Lock()
	defer q.lck.Unlock()
	var res util.String
	item := q.Find(func(tx types.BaseTx, feeRate util.String) bool {
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
	q.actualSize = 0
	q.senderNonceIndex = senderNonces{}
}
