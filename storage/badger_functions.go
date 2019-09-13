package storage

import (
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
)

// BadgerFunctions implements storage.Functions.
type BadgerFunctions struct {
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

// NewBadgerFunctions returns an instance of BadgerFunctions
func NewBadgerFunctions(db *badger.DB, finish, renew bool) *BadgerFunctions {
	return &BadgerFunctions{db: db, tx: db.NewTransaction(true), finish: finish, renew: renew}
}

// CanFinish checks whether transaction is automatically committed
// after every successful operation.
func (f *BadgerFunctions) CanFinish() bool {
	return f.finish
}

// commit commits the transaction if auto commit is enabled
func (f *BadgerFunctions) commit() error {
	if f.finish {
		return f.Commit()
	}
	return nil
}

// Commit commits the transaction
func (f *BadgerFunctions) Commit() error {
	defer f.doRenew()
	return f.tx.Commit()
}

// doRenew renews the transaction only renew and finish are true
func (f *BadgerFunctions) doRenew() {
	f.Lock()
	defer f.Unlock()
	if f.finish && f.renew {
		f.tx = f.db.NewTransaction(true)
	}
}

// Discard discards the transaction
func (f *BadgerFunctions) Discard() {
	defer f.doRenew()
	f.tx.Discard()
}

// Put adds a record to the database.
// It will discard the transaction if an error occurred.
func (f *BadgerFunctions) Put(record *Record) error {
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
func (f *BadgerFunctions) Get(key []byte) (*Record, error) {
	f.Lock()
	defer f.doRenew()
	defer f.Unlock()

	item, err := f.tx.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	val := make([]byte, item.ValueSize())
	if _, err := item.ValueCopy(val); err != nil {
		return nil, errors.Wrap(err, "failed to read value")
	}

	return NewFromKeyValue(key, val), nil
}

// Del deletes a record by key
func (f *BadgerFunctions) Del(key []byte) error {
	f.Lock()
	defer f.commit()
	defer f.Unlock()
	return f.tx.Delete(key)
}

// Iterate finds a set of records by prefix and passes them to iterFunc
// for further processing.
// If iterFunc returns true, the iterator is stopped and immediately released.
// If first is set to true, it begins from the first record, otherwise,
// it will begin from the last record
func (f *BadgerFunctions) Iterate(prefix []byte, first bool, iterFunc func(rec *Record) bool) {
	f.RLock()
	defer f.RUnlock()
	opts := badger.DefaultIteratorOptions
	opts.Reverse = !first
	it := f.tx.NewIterator(opts)
	defer it.Close()
	for it.Seek(prefix); it.Valid(); it.Next() {
		item := it.Item()
		k := item.Key()
		v, _ := item.ValueCopy(nil)
		if iterFunc(NewFromKeyValue(k, v)) {
			return
		}
	}
	return
}
