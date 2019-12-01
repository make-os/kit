package repo

import (
	"fmt"
	"sync"
	"time"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
)

var (
	errTxExistInPushPool = fmt.Errorf("push tx already exist in pool")
	errFullPushPool      = fmt.Errorf("push pool is full")
)

type containerItem struct {
	Tx        *PushTx
	FeeRate   util.String
	TimeAdded time.Time
}

// containerIndex stores tx hashes of transactions in the container
type containerIndex map[string]*containerItem

// adds an entry
func (idx *containerIndex) add(key string, item *containerItem) {
	(*idx)[key] = item
}

// has checks whether a key exist in the index
func (idx *containerIndex) has(key string) bool {
	_, ok := (*idx)[key]
	return ok
}

// get checks whether a key exist in the index
func (idx *containerIndex) get(key string) *containerItem {
	val, _ := (*idx)[key]
	return val
}

// remove removes a key
func (idx *containerIndex) remove(key string) {
	delete(*idx, key)
}

// refNonceIndex stores a mapping of references and the nonce
type refNonceIndex map[string]uint64

func makeRefKey(repo, ref string) string {
	return fmt.Sprintf("%s:%s", ref, repo)
}

// add adds maps a nonce to the given reference
func (i *refNonceIndex) add(refKey string, nonce uint64) {
	(*i)[refKey] = nonce
}

// getNonce returns the nonce for the given ref, or returns 0 if not found
func (i *refNonceIndex) getNonce(refKey string) uint64 {
	nonce, ok := (*i)[refKey]
	if !ok {
		return 0
	}
	return nonce
}

// remove removes a key
func (i *refNonceIndex) remove(refKey string) {
	delete(*i, refKey)
}

// newItem creates an instance of ContainerItem
func newItem(tx *PushTx) *containerItem {
	item := &containerItem{Tx: tx, TimeAdded: time.Now()}
	return item
}

// PushPool implements types.PushPool.
type PushPool struct {
	gmx         *sync.RWMutex
	cap         int
	container   []*containerItem
	index       containerIndex
	refIndex    containerIndex
	refNonceIdx refNonceIndex
}

// NewPushPool creates an instance of PushPool
func NewPushPool(cap int) *PushPool {
	pool := &PushPool{
		gmx:         &sync.RWMutex{},
		cap:         cap,
		container:   []*containerItem{},
		index:       containerIndex(map[string]*containerItem{}),
		refIndex:    containerIndex(map[string]*containerItem{}),
		refNonceIdx: refNonceIndex(map[string]uint64{}),
	}

	tick := time.NewTicker(params.PushPoolCleanUpInt)
	go func() {
		for range tick.C {
			pool.removeOld()
		}
	}()

	return pool
}

// Full returns true if the pool is full
func (p *PushPool) Full() bool {
	p.gmx.RLock()
	isFull := p.cap > 0 && len(p.container) >= p.cap
	p.gmx.RUnlock()
	return isFull
}

// Add a push transaction to the pool.
//
// Check all the references to ensure there are no identical (same repo,
// reference and nonce) references with same nonce in the pool. A valid
// reference is one which has no identical reference with a higher fee rate in
// the pool. If an identical reference exist in the pool with an inferior fee
// rate, the existing tx holding the reference is eligible for replacable by tx
// holding the reference with a superior fee rate. In cases where more than one
// reference of tx is superior to multiple references in multiple transactions,
// replacement will only happen if the fee rate of tx is higher than the
// combined fee rate of the replaceable transactions.
func (p *PushPool) Add(tx types.PushTx) error {

	if p.Full() {
		return errFullPushPool
	}

	p.gmx.Lock()
	defer p.gmx.Unlock()

	id := tx.ID()
	if p.index.has(id.HexStr()) {
		return errTxExistInPushPool
	}

	item := newItem(tx.(*PushTx))

	// Calculate and set fee rate
	billableTxSize := decimal.NewFromFloat(float64(tx.BillableSize()))
	item.FeeRate = util.String(tx.TotalFee().Decimal().Div(billableTxSize).String())

	// Check if references of the transactions are valid
	// or can replace existing transaction
	var replaceable = make(map[string]*PushTx)
	var totalReplaceableFee = decimal.NewFromFloat(0)
	for _, ref := range tx.(*PushTx).References {

		existingRefNonce := p.refNonceIdx.getNonce(makeRefKey(tx.(*PushTx).RepoName, ref.Name))
		if existingRefNonce == 0 {
			continue
		}

		// When the existing reference has a higher nonce, reject tx
		// TODO: Should we support a cache system to hold this tx and later
		// retry it?
		if existingRefNonce < ref.Nonce {
			return fmt.Errorf("rejected because an identical reference with a lesser " +
				"nonce has been staged")
		}

		existingItem := p.refIndex.get(makeRefKey(tx.(*PushTx).RepoName, ref.Name))
		if existingItem == nil {
			panic(fmt.Errorf("unexpectedly failed to find existing reference tx"))
		}

		if existingItem.Tx.TotalFee().Decimal().GreaterThanOrEqual(tx.TotalFee().Decimal()) {
			msg := fmt.Sprintf("replace-by-fee on staged reference (ref:%s, repo:%s) "+
				"not allowed due to inferior fee.", ref.Name, tx.(*PushTx).RepoName)
			return fmt.Errorf(msg)
		}

		txID := existingItem.Tx.ID().String()
		if _, ok := replaceable[txID]; !ok {
			replaceable[txID] = existingItem.Tx
			totalReplaceableFee = totalReplaceableFee.Add(existingItem.Tx.TotalFee().Decimal())
		}
	}

	// Here we need to remove the replaceable transactions. But we will only do so
	// if the total fee of these transactions is lower than that of tx
	if len(replaceable) > 0 {
		if totalReplaceableFee.GreaterThanOrEqual(item.Tx.TotalFee().Decimal()) {
			msg := fmt.Sprintf("replace-by-fee on multiple transactions not " +
				"allowed due to inferior fee.")
			return fmt.Errorf(msg)
		}
		p.remove(funk.Values(replaceable).([]*PushTx)...)
	}

	// Add new tx item to container
	p.container = append(p.container, item)

	// Add indexes for faster queries
	p.index.add(id.HexStr(), item)
	for _, ref := range item.Tx.References {
		p.refIndex.add(makeRefKey(tx.(*PushTx).RepoName, ref.Name), item)
		p.refNonceIdx.add(makeRefKey(tx.(*PushTx).RepoName, ref.Name), ref.Nonce)
	}

	p.broadcast(tx)

	return nil
}

// removeOps removes a transaction from all indexes.
// Note: Not thread safe
func (p *PushPool) removeOps(tx *PushTx) {
	delete(p.index, tx.ID().HexStr())
	for _, ref := range tx.References {
		p.refIndex.remove(makeRefKey(tx.RepoName, ref.Name))
		p.refNonceIdx.remove(makeRefKey(tx.RepoName, ref.Name))
	}
}

// remove removes transactions from the pool
// Note: Not thread-safe.
func (p *PushPool) remove(txs ...*PushTx) {
	finalTxs := funk.Filter(p.container, func(o *containerItem) bool {
		if funk.Find(txs, func(tx *PushTx) bool {
			return o.Tx.ID().Equal(tx.ID())
		}) != nil {
			p.removeOps(o.Tx)
			return false
		}
		return true
	})
	p.container = finalTxs.([]*containerItem)
}

// validate validates a push transaction
func (p *PushPool) validate(tx *PushTx) error {
	return nil
}

// sort sorts the pool
func (p *PushPool) sort() {

}

// broadcast a push transaction
func (p *PushPool) broadcast(tx types.PushTx) error {
	return nil
}

// removeOld finds and removes transactions that
// have stayed up to their TTL in the pool
func (p *PushPool) removeOld() {
	p.gmx.Lock()
	defer p.gmx.Unlock()
	finalTxs := funk.Filter(p.container, func(o *containerItem) bool {
		if time.Now().Sub(o.TimeAdded).Seconds() >= params.PushPoolItemTTL.Seconds() {
			p.removeOps(o.Tx)
			return false
		}
		return true
	})
	p.container = finalTxs.([]*containerItem)
}
