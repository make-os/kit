package pool

import (
	"fmt"
	"sync"
	"time"

	"github.com/make-os/kit/params"
	"github.com/make-os/kit/pkgs/cache"
	"github.com/make-os/kit/remote/push/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
)

var (
	errFullPushPool = fmt.Errorf("push pool is full")
)

// PushPool implements types.PushPool.
type PushPool struct {
	gmx         *sync.RWMutex    // general lock
	cap         int              // The number of transaction the pool is capable of holding.
	container   []*containerItem // Holds all the push notes in the pool
	noteIdx     containerIndex   // Helps keep track of note in the pool
	refIdx      containerIndex   // Helps keep track of note targeting references of a repository
	refNonceIdx refNonceIndex    // Helps keep track of the nonce of note's references
	logic       core.Logic       // The application logic manager
	seen        *cache.Cache     // Helps keep track of notes recently seen; even though they are no longer in the pool
}

// NewPushPool creates an instance of PushPool
func NewPushPool(cap int, logic core.Logic) *PushPool {
	pool := &PushPool{
		gmx:         &sync.RWMutex{},
		cap:         cap,
		container:   []*containerItem{},
		noteIdx:     containerIndex(map[string]*containerItem{}),
		refIdx:      containerIndex(map[string]*containerItem{}),
		refNonceIdx: refNonceIndex(map[string]uint64{}),
		seen:        cache.NewCache(1000),
		logic:       logic,
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

// Add a note to the pool.
//
// Check all the references to ensure there are no identical (same repo,
// reference and nonce) references in the pool. A valid reference is one
// which has no identical reference with a higher fee rate in the pool.
//
// If an identical reference exist in the pool with a lower fee rate, the
// existing note holding the reference will be replaced by new note holding
// the reference with a superior fee rate.
//
// In cases where more than one references of note is superior to multiple
// references in multiple push notes, replacement will only happen if the
// fee rate of note is higher than the combined fee rate of the replaceable
// push notes.
func (p *PushPool) Add(note types.PushNote) error {

	if p.Full() {
		return errFullPushPool
	}

	p.gmx.Lock()
	defer p.gmx.Unlock()

	// Do nothing if note has been added already
	id := note.ID()
	if p.noteIdx.has(id.HexStr()) {
		return nil
	}

	// Create new pool item
	item := newItem(note.(*types.Note))

	// Calculate and set fee rate
	billableTxSize := decimal.NewFromFloat(float64(note.SizeForFeeCal()))
	item.FeeRate = util.String(note.GetFee().Decimal().Div(billableTxSize).String())

	// Check if reference can be added or can cause a note replacement.
	var replaceable = make(map[string]types.PushNote)
	var totalReplaceableFee = decimal.NewFromFloat(0)
	for _, ref := range note.(*types.Note).References {

		existingRefNonce := p.refNonceIdx.getNonce(makeRefKey(note.(*types.Note).RepoName, ref.Name))
		if existingRefNonce == 0 {
			continue
		}

		// When the existing reference has a lower nonce, reject note
		if existingRefNonce < ref.Nonce {
			return fmt.Errorf("rejected: pool has existing reference with a lower nonce")
		}

		existingItem := p.refIdx.get(makeRefKey(note.(*types.Note).RepoName, ref.Name))
		if existingItem == nil {
			panic(fmt.Errorf("unexpectedly failed to find existing reference note"))
		}

		// Reject note if existing reference fee is greater/equal
		if existingItem.Note.GetFee().Decimal().GreaterThanOrEqual(note.GetFee().Decimal()) {
			msg := fmt.Sprintf("replace-by-fee on staged reference (ref:%s, repo:%s) "+
				"not allowed due to inferior fee", ref.Name, note.(*types.Note).RepoName)
			return fmt.Errorf(msg)
		}

		pushNoteID := existingItem.Note.ID().String()
		if _, ok := replaceable[pushNoteID]; !ok {
			replaceable[pushNoteID] = existingItem.Note
			totalReplaceableFee = totalReplaceableFee.Add(existingItem.Note.GetFee().Decimal())
		}
	}

	// Here we need to remove the replaceable push notes. But we will only do so
	// if the total fee of these push notes is lower than that of note
	if len(replaceable) > 0 {
		if totalReplaceableFee.GreaterThanOrEqual(item.Note.GetFee().Decimal()) {
			msg := fmt.Sprintf("replace-by-fee on multiple push notes not allowed due to inferior fee")
			return fmt.Errorf(msg)
		}
		p.remove(funk.Values(replaceable).([]types.PushNote)...)
	}

	// Register new note item to container
	p.container = append(p.container, item)

	// Register indexes for faster queries
	p.noteIdx.add(id.HexStr(), item)
	for _, ref := range item.Note.References {
		p.refIdx.add(makeRefKey(note.(*types.Note).RepoName, ref.Name), item)
		p.refNonceIdx.add(makeRefKey(note.(*types.Note).RepoName, ref.Name), ref.Nonce)
	}

	// Add note to the 'seen' cache
	p.seen.Add(id.String(), struct{}{})

	return nil
}

// HasSeen checks whether a note with the given ID was recently added
func (p *PushPool) HasSeen(noteID string) bool {
	return p.seen.Get(noteID) != nil
}

// removeOps removes a transaction from all indexes.
// Note: Not thread safe
func (p *PushPool) removeOps(note types.PushNote) {
	delete(p.noteIdx, note.ID().HexStr())
	for _, ref := range note.GetPushedReferences() {
		p.refIdx.remove(makeRefKey(note.GetRepoName(), ref.Name))
		p.refNonceIdx.remove(makeRefKey(note.GetRepoName(), ref.Name))
	}
}

// remove removes push notes from the pool
// Note: Not thread-safe.
func (p *PushPool) remove(pushNotes ...types.PushNote) {
	finalTxs := funk.Filter(p.container, func(o *containerItem) bool {
		if funk.Find(pushNotes, func(note types.PushNote) bool {
			return o.Note.ID().Equal(note.ID())
		}) != nil {
			p.removeOps(o.Note)
			return false
		}
		return true
	})
	p.container = finalTxs.([]*containerItem)
}

// Remove removes a push note
func (p *PushPool) Remove(pushNote types.PushNote) {
	p.gmx.Lock()
	defer p.gmx.Unlock()
	p.remove(pushNote)
}

// Get finds and returns a push note
func (p *PushPool) Get(noteID string) *types.Note {
	res := p.noteIdx.get(noteID)
	if res == nil {
		return nil
	}
	return res.Note
}

// removeOld finds and removes push notes that
// have stayed up to their TTL in the pool
func (p *PushPool) removeOld() {
	p.gmx.Lock()
	defer p.gmx.Unlock()
	finalTxs := funk.Filter(p.container, func(o *containerItem) bool {
		if time.Now().Sub(o.TimeAdded).Seconds() >= params.PushPoolItemTTL.Seconds() {
			p.removeOps(o.Note)
			return false
		}
		return true
	})
	p.container = finalTxs.([]*containerItem)
}

// Len returns the number of push notes in the pool
func (p *PushPool) Len() int {
	p.gmx.RLock()
	defer p.gmx.RUnlock()
	return len(p.container)
}

type containerItem struct {
	Note      *types.Note
	FeeRate   util.String
	TimeAdded time.Time
}

// containerIndex stores hashes of push notes in the container
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

// get returns an entry by its key
func (idx *containerIndex) get(key string) *containerItem {
	val, _ := (*idx)[key]
	return val
}

// remove removes a key
func (idx *containerIndex) remove(key string) {
	delete(*idx, key)
}

type repoNotesIndex map[string][]*containerItem

// has checks whether a repo exist in the index
func (idx *repoNotesIndex) has(repo string) bool {
	_, ok := (*idx)[repo]
	return ok
}

// adds an a new entry to a repo's list of push notes
func (idx *repoNotesIndex) add(repo string, item *containerItem) {
	if !idx.has(repo) {
		(*idx)[repo] = []*containerItem{item}
		return
	}
	(*idx)[repo] = append((*idx)[repo], item)
}

// remove removes a note from a repo's list of push notes
func (idx *repoNotesIndex) remove(repo, pushNoteID string) {
	if !idx.has(repo) {
		return
	}

	curTxs := (*idx)[repo]
	curTxs = funk.Filter(curTxs, func(item *containerItem) bool {
		return item.Note.ID().String() != pushNoteID
	}).([]*containerItem)
	(*idx)[repo] = curTxs

	if len(curTxs) == 0 {
		delete(*idx, repo)
	}
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
func newItem(note *types.Note) *containerItem {
	item := &containerItem{Note: note, TimeAdded: time.Now()}
	return item
}
