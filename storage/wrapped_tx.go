package storage

import (
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/make-os/lobe/storage/common"
	"github.com/make-os/lobe/storage/types"
	"github.com/pkg/errors"
)

// WrappedTx implements types.Tx
type WrappedTx struct {
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

// NewBadgerFunctions returns an instance of WrappedTx
func NewBadgerFunctions(db *badger.DB, finish, renew bool) *WrappedTx {
	return &WrappedTx{db: db, tx: db.NewTransaction(true), finish: finish, renew: renew}
}

// GetTx get the underlying transaction
func (f *WrappedTx) GetTx() *badger.Txn {
	return f.tx
}

// NewTx creates a new transaction.
// autoFinish: ensure that the underlying transaction is committed after
// each successful operation.
// renew: reinitialize the transaction after each operation. Requires
// autoFinish to be enabled.
func (f *WrappedTx) NewTx(autoFinish, renew bool) types.Tx {
	return NewBadgerFunctions(f.db, autoFinish, renew)
}

// CanFinish checks whether transaction is automatically committed
// after every successful operation.
func (f *WrappedTx) CanFinish() bool {
	return f.finish
}

// commit commits the transaction if auto commit is enabled
func (f *WrappedTx) commit() error {
	defer f.renewTx()
	if f.finish {
		return f.Commit()
	}
	return nil
}

// Commit commits the transaction
func (f *WrappedTx) Commit() error {
	return f.tx.Commit()
}

// renewTx renews the transaction only if auto renew and auto finish are enabled
func (f *WrappedTx) renewTx() {
	f.Lock()
	defer f.Unlock()
	if f.finish && f.renew {
		f.tx = f.db.NewTransaction(true)
	}
}

// Discard discards the transaction
func (f *WrappedTx) Discard() {
	f.tx.Discard()
}

// Put adds a record to the database.
// It will discard the transaction if an error occurred.
func (f *WrappedTx) Put(record *common.Record) error {
	f.renewTx()
	f.Lock()
	err := f.tx.Set(record.GetKey(), record.Value)
	if err != nil {
		f.Unlock()
		f.Discard()
		return err
	}
	f.Unlock()
	return f.commit()
}

// Get a record by key
func (f *WrappedTx) Get(key []byte) (*common.Record, error) {
	f.renewTx()
	f.Lock()
	defer f.Unlock()

	item, err := f.tx.Get(key)
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
func (f *WrappedTx) RenewTx() {
	f.Lock()
	defer f.Unlock()
	f.tx = f.db.NewTransaction(true)
}

// Del deletes a record by key
func (f *WrappedTx) Del(key []byte) error {
	f.renewTx()
	f.Lock()
	defer f.commit()
	defer f.Unlock()
	return f.tx.Delete(key)
}

// Iterate finds a set of records by prefix and passes them to iterFunc
// for further processing.
//
// If iterFunc returns true, the iterator is stopped and immediately released.
//
// If first is set to true, it begins from the first record, otherwise,
// it will begin from the last record
func (f *WrappedTx) Iterate(prefix []byte, first bool, iterFunc func(rec *common.Record) bool) {
	f.renewTx()
	f.RLock()
	defer f.RUnlock()
	opts := badger.DefaultIteratorOptions
	opts.Reverse = !first
	it := f.tx.NewIterator(opts)
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
	return
}

// RawIterator returns badger's Iterator
func (f *WrappedTx) RawIterator(opts interface{}) interface{} {
	return f.tx.NewIterator(opts.(badger.IteratorOptions))
}

// NewBatch returns a batch writer
func (f *WrappedTx) NewBatch() interface{} {
	return f.db.NewWriteBatch()
}
