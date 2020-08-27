package storage

// Engine provides an interface that describes
// a storage engine
type Engine interface {
	Operations

	// Init initializes the engine. This is where to
	// run and open the storage engine
	// dir: The path to the db file
	Init(dir string) error

	// Close closes the database engine and frees resources
	Close() error
}

// TxCommitDiscarder represents an interface for committing and
// discarding a transaction
type TxCommitDiscarder interface {
	CanFinish() bool
	Commit() error
	Discard()
}

// TxRenewer represents an interface for renewing transaction
type TxRenewer interface {
	RenewTx()
}

// Operations describe the operations of Functions
type Operations interface {

	// Put adds a record to the database.
	// It will discard the transaction if an error occurred.
	Put(record *Record) error

	// Get a record by key
	Get(key []byte) (*Record, error)

	// Del deletes a record by key
	Del(key []byte) error

	// Iterate finds a set of records by prefix and passes them to iterFunc
	// for further processing.
	//
	// If iterFunc returns true, the iteration is stopped and immediately released.
	//
	// If first is set to true, it begins from the first record, otherwise
	// it will begin from the last record
	Iterate(prefix []byte, first bool, iterFunc func(rec *Record) bool)

	// RawIterator returns badger's Iterator
	RawIterator(opts interface{}) interface{}

	// NewBatch returns a batch writer
	NewBatch() interface{}

	// NewTx creates a new transaction.
	// autoFinish: ensure that the underlying transaction is committed after
	// each successful operation.
	// renew: reinitializes the transaction after each operation. Requires
	// autoFinish to be enabled.
	NewTx(autoFinish, renew bool) Tx
}

// Tx describes a transaction
type Tx interface {
	TxCommitDiscarder
	Operations
	TxRenewer
}
