package storage

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/make-os/lobe/storage/common"
	"github.com/make-os/lobe/storage/types"
	"github.com/pkg/errors"
)

// ErrRecordNotFound indicates that a record was not found
var ErrRecordNotFound = fmt.Errorf("record not found")

// Badger implements storagetypes.Engine. It provides
// storage functions built on top of badger.
type Badger struct {
	*WrappedTx
	db *badger.DB
}

// NewBadger creates an instance of Badger storage engine.
func NewBadger() *Badger {
	return &Badger{}
}

// Init starts the database.
// If dir is empty, an in-memory DB is created.
func (b *Badger) Init(dir string) error {

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
	b.WrappedTx = NewBadgerFunctions(db, true, true)

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
	return NewBadgerFunctions(b.db, autoFinish, renew)
}

// Close closes the database engine and frees resources
func (b *Badger) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
