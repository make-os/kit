package storage

import (
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/make-os/kit/storage/common"
	"github.com/make-os/kit/storage/types"
	"github.com/pkg/errors"
)

// Tx implements types.Tx
type Tx struct {
	sync.RWMutex

	// db is the badger database
	db *badger.DB

	// tx is the badger transaction
	tx *badger.Txn

	// finish determines whether commit is automatically called
	// after a successful operation
	finish bool

	// renew determines whether the tx is renewed after successful
	// commit/discard
	renew bool
}

// NewTx returns an instance of Tx
func NewTx(db *badger.DB, finish, renew bool) *Tx {
	return &Tx{db: db, tx: db.NewTransaction(true), finish: finish, renew: renew}
}

// GetTx get the underlying transaction
func (t *Tx) GetTx() *badger.Txn {
	t.Lock()
	defer t.Unlock()
	return t.tx
}

// NewTx creates a new transaction.
// autoFinish: ensure that the underlying transaction is committed after
// each successful operation.
// renew: reinitialize the transaction after each operation. Requires
// autoFinish to be enabled.
func (t *Tx) NewTx(autoFinish, renew bool) types.Tx {
	return NewTx(t.db, autoFinish, renew)
}

// CanFinish checks whether transaction is automatically committed
// after every successful operation.
func (t *Tx) CanFinish() bool {
	t.RLock()
	defer t.RUnlock()
	return t.finish
}

// commit commits the transaction if auto commit is enabled
func (t *Tx) commit() error {
	defer t.renewTx()

	t.RLock()
	finished := t.finish
	t.RUnlock()

	if finished {
		return t.Commit()
	}

	return nil
}

// Commit commits the transaction
func (t *Tx) Commit() error {
	t.Lock()
	defer t.Unlock()
	return t.tx.Commit()
}

// renewTx renews the transaction only if auto renew and auto finish are enabled
func (t *Tx) renewTx() {
	t.Lock()
	defer t.Unlock()
	if t.finish && t.renew {
		t.tx = t.db.NewTransaction(true)
	}
}

// Discard discards the transaction
func (t *Tx) Discard() {
	t.Lock()
	defer t.Unlock()
	t.tx.Discard()
}

// Put adds a record to the database.
// It will discard the transaction if an error occurred.
func (t *Tx) Put(record *common.Record) error {
	t.renewTx()
	t.Lock()
	err := t.tx.Set(record.GetKey(), record.Value)
	if err != nil {
		t.Unlock()
		t.Discard()
		return err
	}
	t.Unlock()
	return t.commit()
}

// Get a record by key
func (t *Tx) Get(key []byte) (*common.Record, error) {
	t.renewTx()
	t.Lock()
	defer t.Unlock()

	item, err := t.tx.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	val := make([]byte, item.ValueSize())
	if val, err = item.ValueCopy(val); err != nil {
		return nil, errors.Wrap(err, "failed to read value")
	}

	return common.NewFromKeyValue(key, val), nil
}

// RenewTx forcefully renews the underlying transaction.
func (t *Tx) RenewTx() {
	t.Lock()
	defer t.Unlock()
	t.tx = t.db.NewTransaction(true)
}

// Del deletes a record by key
func (t *Tx) Del(key []byte) error {
	t.renewTx()
	defer t.commit()

	t.Lock()
	if err := t.tx.Delete(key); err != nil {
		t.Unlock()
		return err
	}
	t.Unlock()

	return nil
}

// Iterate finds a set of records by prefix and passes them to iterFunc
// for further processing.
//
// If iterFunc returns true, the iterator is stopped and immediately released.
//
// If first is set to true, it begins from the first record, otherwise,
// it will begin from the last record
func (t *Tx) Iterate(prefix []byte, first bool, iterFunc func(rec *common.Record) bool) {
	t.renewTx()
	opts := badger.DefaultIteratorOptions
	opts.Reverse = !first
	opts.PrefetchSize = 1000

	t.Lock()
	it := t.tx.NewIterator(opts)
	t.Unlock()
	defer it.Close()

	var prefixKey = append([]byte{}, prefix...)
	if opts.Reverse {
		prefixKey = append(prefixKey, 0xFF)
	}

	for it.Seek(prefixKey); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		k := item.Key()
		v, _ := item.ValueCopy(nil)
		if iterFunc(common.NewFromKeyValue(k, v)) {
			return
		}
	}
}

// RawIterator returns badger's Iterator
func (t *Tx) RawIterator(opts interface{}) interface{} {
	t.Lock()
	defer t.Unlock()
	return t.tx.NewIterator(opts.(badger.IteratorOptions))
}

// NewBatch returns a batch writer
func (t *Tx) NewBatch() interface{} {
	return t.db.NewWriteBatch()
}
