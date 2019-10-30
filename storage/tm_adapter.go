package storage

import (
	"bytes"

	"github.com/dgraph-io/badger"
	tmdb "github.com/tendermint/tm-db"
)

// TMDBAdapter fully implements some of github.com/tendermint/tm-db.DB interface.
type TMDBAdapter struct {
	db Tx
}

// NewTMDBAdapter creates an instance TMDBAdapter.
func NewTMDBAdapter(db Tx) *TMDBAdapter {
	return &TMDBAdapter{db: db}
}

// Get returns nil if key doesn't exist.
// A nil key is interpreted as an empty byteslice.
// CONTRACT: key, value readonly []byte
func (tm *TMDBAdapter) Get(k []byte) []byte {
	rec, err := tm.db.Get(k)
	if err != nil {
		if err == ErrRecordNotFound {
			return nil
		}
		panic(err)
	}
	return rec.Value
}

// Has checks if a key exists.
// A nil key is interpreted as an empty byteslice.
// CONTRACT: key, value readonly []byte
func (tm *TMDBAdapter) Has(key []byte) bool {
	return tm.Get(key) != nil
}

// Set sets the key.
// A nil key is interpreted as an empty byteslice.
// CONTRACT: key, value readonly []byte
func (tm *TMDBAdapter) Set(k []byte, v []byte) {
	tm.db.Put(NewFromKeyValue(k, v))
}

// SetSync is like Set but works synchronously
func (tm *TMDBAdapter) SetSync([]byte, []byte) {
	panic("not implemented")
}

// Delete deletes the key.
// A nil key is interpreted as an empty byteslice.
// CONTRACT: key readonly []byte
func (tm *TMDBAdapter) Delete(k []byte) {
	tm.db.Del(k)
}

// DeleteSync is like Delete but works synchronously
func (tm *TMDBAdapter) DeleteSync([]byte) {
	panic("not implemented")
}

// Iterator iterates over a domain of keys in ascending order. End is exclusive.
// Start must be less than end, or the Iterator is invalid.
// A nil start is interpreted as an empty byteslice.
// If end is nil, iterates up to the last item (inclusive).
// CONTRACT: No writes may happen within a domain while an iterator exists over it.
// CONTRACT: start, end readonly []byte
// CONTRACT: Runs in a new managed transaction
func (tm *TMDBAdapter) Iterator(start, end []byte) tmdb.Iterator {
	return NewTMDBIteratorAdapter(tm.db.NewTx(true, true), start, end, false)
}

// ReverseIterator iterate over a domain of keys in descending order. End is exclusive.
// Start must be less than end, or the Iterator is invalid.
// If start is nil, iterates up to the first/least item (inclusive).
// If end is nil, iterates from the last/greatest item (inclusive).
// CONTRACT: No writes may happen within a domain while an iterator exists over it.
// CONTRACT: start, end readonly []byte
// CONTRACT: Runs in a new managed transaction.
func (tm *TMDBAdapter) ReverseIterator(start, end []byte) tmdb.Iterator {
	return NewTMDBIteratorAdapter(tm.db.NewTx(true, true), start, end, true)
}

// Close closes the connection.
func (tm *TMDBAdapter) Close() {
	return
}

// NewBatch creates a batch for atomic updates.
func (tm *TMDBAdapter) NewBatch() tmdb.Batch {
	return &TMDBBatchAdapter{
		bw: tm.db.NewBatch().(*badger.WriteBatch),
	}
}

// Print is for debugging
func (tm *TMDBAdapter) Print() {
	panic("not implemented")
}

// Stats returns a map of property values for all keys and the size of the cache.
func (tm *TMDBAdapter) Stats() map[string]string {
	return nil
}

// TMDBBatchAdapter implements github.com/tendermint/tm-db.Batch
type TMDBBatchAdapter struct {
	bw *badger.WriteBatch
	m  [][][]byte
}

// Write writes to the batch writer
func (b *TMDBBatchAdapter) Write() {
	if err := b.bw.Flush(); err != nil {
		panic(err.Error())
	}
	return
}

// WriteSync is like Write, but synchronous
func (b *TMDBBatchAdapter) WriteSync() {
	panic("not implemented")
}

// Close cancels the batch writer
func (b *TMDBBatchAdapter) Close() {
	b.bw.Cancel()
}

// Set adds a set of key and value pair to the batch
func (b *TMDBBatchAdapter) Set(key, value []byte) {
	b.m = append(b.m, [][]byte{key, value})
	b.bw.Set(key, value)
}

// Delete removes a key from the batch
func (b *TMDBBatchAdapter) Delete(key []byte) {
	b.bw.Delete(key)
}

// TMDBIteratorAdapter implements github.com/tendermint/tm-db.Iterator
type TMDBIteratorAdapter struct {
	db      Tx
	k       []byte
	v       []byte
	valid   bool
	it      *badger.Iterator
	start   []byte
	end     []byte
	reverse bool
}

// NewTMDBIteratorAdapter returns an instance of TMDBIteratorAdapter
func NewTMDBIteratorAdapter(db Tx, start, end []byte, reverse bool) *TMDBIteratorAdapter {
	iOpts := badger.DefaultIteratorOptions
	iOpts.Reverse = reverse
	it := db.RawIterator(iOpts).(*badger.Iterator)

	// Configure the iterator based on the provide
	// start, end and reverse mode parameters.
	if !reverse {
		// move cursor to the beginning
		it.Seek(start)
	} else {
		// when no end key is provided, move
		// cursor to the end
		if end == nil {
			it.Rewind()
		} else {
			// when end key is provided, move cursor to the
			// end key. If the iterator is still valid and the
			// end key is less than or equal to the current item's key
			// move one step forward such that the end key is excluded.
			it.Seek(end)
			if it.Valid() {
				keyAtCursor := it.Item().Key()
				if bytes.Compare(end, keyAtCursor) <= 0 {
					it.Next()
				}
			} else {
				it.Rewind()
			}
		}
	}

	return &TMDBIteratorAdapter{
		db:      db,
		it:      it,
		start:   start,
		end:     end,
		reverse: reverse,
	}
}

// Domain returns the start & end (exclusive) limits to iterate over.
// If end < start, then the Iterator goes in reverse order.
//
// A domain of ([]byte{12, 13}, []byte{12, 14}) will iterate
// over anything with the prefix []byte{12, 13}.
//
// The smallest key is the empty byte array []byte{} - see BeginningKey().
// The largest key is the nil byte array []byte(nil) - see EndingKey().
// CONTRACT: start, end readonly []byte
func (it *TMDBIteratorAdapter) Domain() (start []byte, end []byte) {
	return it.k, it.v
}

// Valid returns whether the current position is valid.
// Once invalid, an Iterator is forever invalid.
func (it *TMDBIteratorAdapter) Valid() bool {

	if !it.it.Valid() {
		return false
	}
	// return tmdb.IsKeyInDomain(it.it.Item().Key(), it.start, it.end)

	key := it.it.Item().Key()

	// In reverse mode, we must stop the iterator
	// when we encounter a key less than the start key.
	if it.reverse {
		if it.start != nil && bytes.Compare(key, it.start) < 0 {
			return false
		}
	} else {
		if it.end != nil && bytes.Compare(it.end, key) <= 0 {
			return false
		}
	}

	return true
}

// Next moves the iterator to the next sequential key in the database, as
// defined by order of iteration.
//
// If Valid returns false, this method will panic.
func (it *TMDBIteratorAdapter) Next() {
	it.it.Next()
}

// Key returns the key of the cursor.
// If Valid returns false, this method will panic.
// CONTRACT: key readonly []byte
func (it *TMDBIteratorAdapter) Key() (key []byte) {
	return it.it.Item().Key()
}

// Value returns the value of the cursor.
// If Valid returns false, this method will panic.
// CONTRACT: value readonly []byte
func (it *TMDBIteratorAdapter) Value() (value []byte) {
	var val = make([]byte, it.it.Item().ValueSize())
	val, _ = it.it.Item().ValueCopy(val)
	return val
}

// Close releases the Iterator.
func (it *TMDBIteratorAdapter) Close() {
	it.it.Close()
}
