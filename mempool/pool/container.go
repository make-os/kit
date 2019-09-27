package pool

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/thoas/go-funk"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
)

var (
	// ErrContainerFull is an error about a full container
	ErrContainerFull = fmt.Errorf("container is full")

	// ErrTxAlreadyAdded is an error about a transaction
	// that is in the pool.
	ErrTxAlreadyAdded = fmt.Errorf("exact transaction already in the pool")

	// ErrFailedReplaceByFeeRate means an attempt to replace by fee failed due to the replacement
	// tx having a lower/equal fee rate to the current
	ErrFailedReplaceByFeeRate = fmt.Errorf("failed to replace transaction at same nonce due to " +
		"low/equal fee rate. Fee rate must be higher to replace the existing transaction")
)

// ContainerItem represents the a container item.
// It wraps a transaction and other important properties.
type ContainerItem struct {
	Tx      types.Tx
	FeeRate util.String
}

// newItem creates an instance of ContainerItem
func newItem(tx types.Tx) *ContainerItem {
	item := &ContainerItem{Tx: tx}
	return item
}

// nonceInfo stores information about a transaction
// that is associated with a specific nonce
type nonceInfo struct {
	TxHash  util.Hash
	FeeRate util.String
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

// add adds a nonce information. If a matching nonce
// already exist, it is replaced with the new nonce info.
func (c *nonceCollection) add(nonce uint64, ni *nonceInfo) {
	c.nonces[nonce] = ni
}

// remove removes a nonce information.
func (c *nonceCollection) remove(nonce uint64) {
	delete(c.nonces, nonce)
}

type senderNonces map[util.String]*nonceCollection

// remove removes a nonce associated with a sender address.
// The entire map entry for the sender is removed if no other
// nonce exist after the operation
func (sn *senderNonces) remove(senderAddr util.String, nonce uint64) {
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
	gmx        *sync.RWMutex
	container  []*ContainerItem       // main transaction container (the pool)
	cap        int64                  // cap is the amount of transactions in the
	len        int64                  // length of the container
	noSorting  bool                   // noSorting indicates that sorting is enabled/disabled
	hashIndex  map[string]interface{} // hashIndex caches tx hashes for quick existence lookup
	byteSize   int64                  // byteSize is the total txs size of the container (excluding fee field)
	actualSize int64                  // actualSize is the total tx size of the container (includes all fields)

	// nonceInfo maps sending addresses to nonces of transaction.
	// We use this to find transactions from same sender with
	// matching nonce for implementing a replay-by-fee mechanism.
	nonceInfo senderNonces
}

// newTxContainer creates a new container
func newTxContainer(cap int64) *TxContainer {
	q := new(TxContainer)
	q.container = []*ContainerItem{}
	q.cap = cap
	q.gmx = &sync.RWMutex{}
	q.hashIndex = map[string]interface{}{}
	q.nonceInfo = map[util.String]*nonceCollection{}
	return q
}

// NewQueueNoSort creates a new container
// with sorting turned off
func NewQueueNoSort(cap int64) *TxContainer {
	q := new(TxContainer)
	q.container = []*ContainerItem{}
	q.cap = cap
	q.gmx = &sync.RWMutex{}
	q.hashIndex = map[string]interface{}{}
	q.noSorting = true
	q.nonceInfo = map[util.String]*nonceCollection{}
	return q
}

// Size returns the number of items in the container
func (q *TxContainer) Size() int64 {
	q.gmx.RLock()
	defer q.gmx.RUnlock()
	return q.len
}

// ByteSize gets the total byte size of
// all transactions in the container.
// Note: The size of fee field of transactions are not calculated.
func (q *TxContainer) ByteSize() int64 {
	return q.byteSize
}

// ActualSize is like ByteSize but its result includes all fields
func (q *TxContainer) ActualSize() int64 {
	return q.actualSize
}

// Full checks if the container's capacity has been reached
func (q *TxContainer) Full() bool {
	q.gmx.RLock()
	defer q.gmx.RUnlock()
	return q.len >= q.cap
}

func (q *TxContainer) noSort() bool {
	q.gmx.RLock()
	defer q.gmx.RUnlock()
	return q.noSorting
}

// add adds a transaction to the end of the container.
// Returns false if container capacity has been reached.
// It computes the fee rate and sorts the transactions
// after addition.
func (q *TxContainer) add(tx types.Tx) error {

	if q.Full() {
		return ErrContainerFull
	}

	item := newItem(tx)

	// Calculate the transaction's fee rate
	// formula: tx fee / size
	txSizeDec := decimal.NewFromBigInt(new(big.Int).SetInt64(tx.GetSizeNoFee()), 0)
	item.FeeRate = util.String(tx.GetFee().Decimal().Div(txSizeDec).String())

	q.gmx.Lock()

	sender := tx.GetFrom()

	// Get the sender's nonce info. If not found create a new one
	senderNonceInfo, ok := q.nonceInfo[sender]
	if !ok {
		senderNonceInfo = defaultNonceCollection()
	}

	// If there is no existing transaction with a matching nonce,
	// add this tx nonce to the collection and proceed to adding
	// the transaction to the container
	if !senderNonceInfo.has(tx.GetNonce()) {
		senderNonceInfo.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash(), FeeRate: item.FeeRate})
	} else {
		// However, reject a transaction if their is already a matching
		// nonce from same sender that has an equal or higher fee rate.
		// CONTRACT: To replace-by-fee, the new transaction must have a higher fee rate.
		ni := senderNonceInfo.get(tx.GetNonce())
		if ni.FeeRate.Decimal().
			GreaterThanOrEqual(item.FeeRate.Decimal()) {
			q.gmx.Unlock()
			return ErrFailedReplaceByFeeRate
		}

		// At the point, the new transaction has a higher fee rate, therefore we
		// need to remove the existing transaction and replace with the new one
		// and also add also replace the nonce information
		q.removeByHash(ni.TxHash)
		senderNonceInfo.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash(), FeeRate: item.FeeRate})
	}

	q.nonceInfo[sender] = senderNonceInfo
	q.container = append(q.container, item)
	q.hashIndex[tx.GetHash().HexStr()] = struct{}{}
	q.len++
	q.byteSize += tx.GetSizeNoFee()
	q.actualSize += int64(len(tx.Bytes()))

	q.gmx.Unlock()

	if !q.noSort() {
		q.Sort()
	}

	return nil
}

// Has checks whether a transaction is in the container
func (q *TxContainer) Has(tx types.Tx) bool {
	q.gmx.RLock()
	defer q.gmx.RUnlock()
	return q.hashIndex[tx.GetHash().HexStr()] != nil
}

// HasByHash is like Has but accepts a transaction hash
func (q *TxContainer) HasByHash(hash string) bool {
	q.gmx.RLock()
	defer q.gmx.RUnlock()
	return q.hashIndex[hash] != nil
}

// First returns a single transaction at head.
// Returns nil if container is empty
func (q *TxContainer) First() types.Tx {

	if q.Size() <= 0 {
		return nil
	}

	q.gmx.Lock()
	defer q.gmx.Unlock()

	item := q.container[0]
	q.container = q.container[1:]
	delete(q.hashIndex, item.Tx.GetHash().HexStr())
	q.nonceInfo.remove(item.Tx.GetFrom(), item.Tx.GetNonce())
	q.byteSize -= item.Tx.GetSizeNoFee()
	q.actualSize -= int64(len(item.Tx.Bytes()))
	q.len--
	return item.Tx
}

// Last returns a single transaction at head.
// Returns nil if container is empty
func (q *TxContainer) Last() types.Tx {

	if q.Size() <= 0 {
		return nil
	}

	q.gmx.Lock()
	defer q.gmx.Unlock()

	lastIndex := len(q.container) - 1
	item := q.container[lastIndex]
	q.container = q.container[0:lastIndex]
	delete(q.hashIndex, item.Tx.GetHash().HexStr())
	q.nonceInfo.remove(item.Tx.GetFrom(), item.Tx.GetNonce())
	q.byteSize -= item.Tx.GetSizeNoFee()
	q.actualSize -= int64(len(item.Tx.Bytes()))
	q.len--
	return item.Tx
}

// Sort sorts the container
func (q *TxContainer) Sort() {
	q.gmx.Lock()
	defer q.gmx.Unlock()
	sort.Slice(q.container, func(i, j int) bool {

		// When transaction i & j belongs to same sender
		// Sort by nonce in ascending order when the nonces are not the same.
		// When they are the same, we sort by the highest fee rate
		if q.container[i].Tx.GetFrom() == q.container[j].Tx.GetFrom() {
			if q.container[i].Tx.GetNonce() < q.container[j].Tx.GetNonce() {
				return true
			}
			if q.container[i].Tx.GetNonce() == q.container[j].Tx.GetNonce() &&
				q.container[i].FeeRate.Decimal().GreaterThan(q.container[j].FeeRate.Decimal()) {
				return true
			}
			return false
		}

		// For other transactions, sort by highest fee rate
		return q.container[i].FeeRate.Decimal().
			GreaterThan(q.container[j].FeeRate.Decimal())
	})
}

// Find iterates over the transactions and invokes iteratee for
// each transaction. The iteratee is invoked the transaction as the
// only argument. It immediately stops and returns the last retrieved
// transaction when the iteratee returns true.
func (q *TxContainer) Find(iteratee func(types.Tx) bool) types.Tx {
	q.gmx.Lock()
	defer q.gmx.Unlock()
	for _, item := range q.container {
		if iteratee(item.Tx) {
			return item.Tx
		}
	}
	return nil
}

// remove removes transactions.
// Note: Not thread-safe
func (q *TxContainer) remove(txs ...types.Tx) {
	finalTxs := funk.Filter(q.container, func(o *ContainerItem) bool {
		if funk.Find(txs, func(tx types.Tx) bool {
			return o.Tx.GetHash().Equal(tx.GetHash())
		}) != nil {
			delete(q.hashIndex, o.Tx.GetHash().HexStr())
			q.nonceInfo.remove(o.Tx.GetFrom(), o.Tx.GetNonce())
			q.byteSize -= o.Tx.GetSizeNoFee()
			q.actualSize -= int64(len(o.Tx.Bytes()))
			q.len--
			return false
		}
		return true
	})

	q.container = finalTxs.([]*ContainerItem)
}

// removeByHash removes transactions by hash
// Note: Not thread-safe
func (q *TxContainer) removeByHash(txsHash ...util.Hash) {
	finalTxs := funk.Filter(q.container, func(o *ContainerItem) bool {
		if funk.Find(txsHash, func(hash util.Hash) bool {
			return o.Tx.GetHash().Equal(hash)
		}) != nil {
			delete(q.hashIndex, o.Tx.GetHash().HexStr())
			q.nonceInfo.remove(o.Tx.GetFrom(), o.Tx.GetNonce())
			q.byteSize -= o.Tx.GetSizeNoFee()
			q.actualSize -= int64(len(o.Tx.Bytes()))
			q.len--
			return false
		}
		return true
	})

	q.container = finalTxs.([]*ContainerItem)
}

// Remove removes a transaction
func (q *TxContainer) Remove(txs ...types.Tx) {
	q.gmx.Lock()
	defer q.gmx.Unlock()
	q.remove(txs...)
}

// GetByHash get a transaction by its hash from the pool
func (q *TxContainer) GetByHash(hash string) types.Tx {
	for _, item := range q.container {
		if hash == item.Tx.GetHash().HexStr() {
			return item.Tx
		}
	}
	return nil
}
