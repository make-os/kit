package storage

import (
	"github.com/dgraph-io/badger"
	"github.com/makeos/mosdef/config"
	"github.com/pkg/errors"
)

// Badger implements storage.Engine. It provides
// storage functions built on top of badger.
type Badger struct {
	*BadgerFunctions
	cfg *config.EngineConfig
	db  *badger.DB
}

// NewBadger creates an instance of Badger storage engine.
func NewBadger(cfg *config.EngineConfig) *Badger {
	return &Badger{cfg: cfg}
}

// Init starts the database
func (b *Badger) Init() error {

	opts := badger.DefaultOptions(b.cfg.NetDataDir())
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

// F returns the functions the engine is capable of executing.
// The argument autoFinish ensure that the underlying transaction
// is committed after each successful operation.
// The argument renew reinitializes the transaction after
// only when autoFinish is true.
func (b *Badger) F(autoFinish, renew bool) Functions {
	return NewBadgerFunctions(b.db, autoFinish, renew)
}

// Close closes the database engine and frees resources
func (b *Badger) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
