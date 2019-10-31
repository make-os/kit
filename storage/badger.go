package storage

import (
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
)

// Badger implements storage.Engine. It provides
// storage functions built on top of badger.
type Badger struct {
	*BadgerFunctions
	db *badger.DB
}

// NewBadger creates an instance of Badger storage engine.
func NewBadger() *Badger {
	return &Badger{}
}

// Init starts the database
func (b *Badger) Init(dir string) error {

	opts := badger.DefaultOptions(dir)
	opts.Logger = &noLogger{}
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
	b.BadgerFunctions = NewBadgerFunctions(db, true, true)

	return nil
}

// GetDB get the underlying db
func (b *Badger) GetDB() *badger.DB {
	return b.db
}

// NewTx creates a new transaction.
// autoFinish: ensure that the underlying transaction is committed after
// each successful operation.
// renew: reinitializes the transaction after each operation. Requires
// autoFinish to be enabled.
func (b *Badger) NewTx(autoFinish, renew bool) Tx {
	return NewBadgerFunctions(b.db, autoFinish, renew)
}

// Close closes the database engine and frees resources
func (b *Badger) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
