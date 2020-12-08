package storage

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/make-os/kit/storage/common"
	"github.com/make-os/kit/storage/types"
	"github.com/pkg/errors"
)

// ErrRecordNotFound indicates that a record was not found
var ErrRecordNotFound = fmt.Errorf("record not found")

// BadgerStore implements storagetypes.Engine. It provides
// storage functions built on top of badger.
type BadgerStore struct {
	*Tx
	lck    *sync.Mutex
	db     *badger.DB
	closed bool
}

// Init starts the database.
// If dir is unset, an in-memory DB is initialized.
func (b *BadgerStore) init(dir string) error {

	opts := badger.DefaultOptions(dir)
	if dir == "" {
		opts = opts.WithInMemory(true)
	}
	opts.Logger = &common.NoopLogger{}
	db, err := badger.Open(opts)
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}

	// Set the database
	b.db = db

	// Initialize the default transaction that auto commits
	// on success ops or discards on failure.
	// It also enables the renewal of the underlying transaction
	// after executing a read/write operation
	b.Tx = NewTx(db, true, true)

	return nil
}

// GetDB get the underlying db
func (b *BadgerStore) GetDB() *badger.DB {
	return b.db
}

// NewTx creates a new transaction.
//
// autoFinish: ensure that the underlying transaction is committed after
// each successful operation.
//
// renew: re-initializes the transaction after each operation. Requires
// autoFinish to be enabled.
func (b *BadgerStore) NewTx(autoFinish, renew bool) types.Tx {
	return NewTx(b.db, autoFinish, renew)
}

// Closed checks whether the DB has been closed
func (b *BadgerStore) Closed() bool {
	b.lck.Lock()
	defer b.lck.Unlock()
	return b.closed
}

// Close closes the database engine and frees resources
func (b *BadgerStore) Close() error {
	b.lck.Lock()
	defer b.lck.Unlock()
	if b.db != nil {
		b.closed = true
		return b.db.Close()
	}
	return nil
}
