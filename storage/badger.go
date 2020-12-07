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

// Badger implements storagetypes.Engine. It provides
// storage functions built on top of badger.
type Badger struct {
	*WrappedTx
	lck    *sync.Mutex
	db     *badger.DB
	closed bool
}

// NewBadger creates an instance of Badger storage engine.
func NewBadger() *Badger {
	return &Badger{lck: &sync.Mutex{}}
}

// Init starts the database.
// If dir is empty, an in-memory DB is created.
func (b *Badger) Init(dir string) error {

	opts := badger.DefaultOptions(dir)
	opts = opts.WithTruncate(true)
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
	b.WrappedTx = NewTx(db, true, true)

	return nil
}

// GetDB get the underlying db
func (b *Badger) GetDB() *badger.DB {
	return b.db
}

// NewTx creates a new transaction.
//
// autoFinish: ensure that the underlying transaction is committed after
// each successful operation.
//
// renew: re-initializes the transaction after each operation. Requires
// autoFinish to be enabled.
func (b *Badger) NewTx(autoFinish, renew bool) types.Tx {
	return NewTx(b.db, autoFinish, renew)
}

// Closed checks whether the DB has been closed
func (b *Badger) Closed() bool {
	b.lck.Lock()
	defer b.lck.Unlock()
	return b.closed
}

// Close closes the database engine and frees resources
func (b *Badger) Close() error {
	b.lck.Lock()
	defer b.lck.Unlock()
	if b.db != nil {
		b.closed = true
		return b.db.Close()
	}
	return nil
}
