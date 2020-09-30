package pool

import (
	"fmt"
	"sync"
	"time"

	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/pkgs/cache"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
)

// containerItem represents the a container item.
// It wraps a transaction and other important properties.
type containerItem struct {
	Tx        types.BaseTx
	FeeRate   util.String
	TimeAdded time.Time
}

// newItem creates an instance of ContainerItem
func newItem(tx types.BaseTx) *containerItem {
	item := &containerItem{Tx: tx, TimeAdded: time.Now()}
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

// newNonceCollection returns a base nonceCollection instance
func newNonceCollection() *nonceCollection {
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

// senderNonces maps a sender's address to a collection of nonce
type senderNonces map[identifier.Address]*nonceCollection

// get finds a nonces associated with the given address.
// Returns empty nonceCollection if no record for the address.
func (sn *senderNonces) get(addr identifier.Address) *nonceCollection {
	if nc, ok := (*sn)[addr]; ok {
		return nc
	}
	return newNonceCollection()
}

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

// NonceGetterFunc describes a function for getting the nonce of an account matching the given address
type NonceGetterFunc func(addr string) (uint64, error)

// Cache describes a queue for transactions
type Cache struct {
	lck            *sync.Mutex
	c              chan types.BaseTx
	senderNonceIdx senderNonces
	firstSeen      *cache.Cache
}

// NewCache creates an instance of Cache
func NewCache() *Cache {
	return &Cache{
		lck:            &sync.Mutex{},
		c:              make(chan types.BaseTx, 10000),
		senderNonceIdx: make(map[identifier.Address]*nonceCollection),
		firstSeen:      cache.NewCache(10000),
	}
}

// Add adds a tx.
// Returns true if the tx was added to the cache.
// Returns false:
//  - If there is an existing transaction from same sender and nonce exists.
//  - If the transaction has spent more than MempoolTxTTL in the cache.
func (c *Cache) Add(tx types.BaseTx) error {
	if c.Has(tx) {
		return fmt.Errorf("cache already contains a transaction with matching sender and nonce")
	}

	// Check if the same tx has been added to the cache before
	// and if the time difference between now and when it was first
	// added exceed the MempoolTxTTL - If yes, return false
	firstSeen := c.getFirstSeen(tx.GetID())
	if !firstSeen.IsZero() {
		expiryTime := firstSeen.Add(params.MempoolTxTTL)
		if time.Now().After(expiryTime) {
			c.firstSeen.Remove(tx.GetID())
			return fmt.Errorf("refused to cache old transaction")
		}
	}
	c.markFirstSeenTime(tx.GetID())

	c.c <- tx

	c.lck.Lock()
	defer c.lck.Unlock()

	senderNonces := c.senderNonceIdx.get(tx.GetFrom())
	senderNonces.add(tx.GetNonce(), &nonceInfo{})
	c.senderNonceIdx[tx.GetFrom()] = senderNonces

	return nil
}

// markFirstSeenTime records the first time the tx hash was firstSeen
func (c *Cache) markFirstSeenTime(hash string) {
	val := c.firstSeen.Get(hash)
	if val == nil {
		c.firstSeen.Add(hash, time.Now().UnixNano())
	}
}

// getSeenMarks returns the last time the tx hash was seen
func (c *Cache) getFirstSeen(hash string) time.Time {
	count := c.firstSeen.Get(hash)
	if count == nil {
		return time.Time{}
	}
	return time.Unix(0, count.(int64))
}

// SizeByAddr returns the number of transactions signed by a given address
func (c *Cache) SizeByAddr(addr identifier.Address) int {
	c.lck.Lock()
	defer c.lck.Unlock()
	return len(c.senderNonceIdx.get(addr).nonces)
}

// Size returns the size of the cache
func (c *Cache) Size() int {
	c.lck.Lock()
	defer c.lck.Unlock()
	return len(c.c)
}

// Get returns a tx from the cache
func (c *Cache) Get() types.BaseTx {
	select {
	case tx := <-c.c:
		c.lck.Lock()
		c.senderNonceIdx.remove(tx.GetFrom(), tx.GetNonce())
		c.lck.Unlock()
		return tx
	default:
		return nil
	}
}

// Has checks if a tx with matching sender address and nonce exist in the cache
func (c *Cache) Has(tx types.BaseTx) bool {
	c.lck.Lock()
	defer c.lck.Unlock()
	if nc, ok := c.senderNonceIdx[tx.GetFrom()]; ok {
		return nc.has(tx.GetNonce())
	}
	return false
}
