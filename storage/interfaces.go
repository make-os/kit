package storage

// Engine provides an interface that describes
// a storage engine
type Engine interface {
	Operations

	// Init initializes the engine. This is where to
	// run and open the storage engine
	Init() error

	// Close closes the database engine and frees resources
	Close() error

	// F returns the functions the engine is capable of executing.
	// Each call to this function returns storage.Functions with a unique
	// transaction.
	// The argument autoFinish ensure that the underlying transaction
	// is committed after each successful operation.
	// The argument renew reinitializes the transaction after
	// only when autoFinish is true.
	F(autoFinish, renew bool) Functions
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
	Put(record *Record) error
	Get(key []byte) (*Record, error)
	Del(key []byte) error
	Iterate(prefix []byte, first bool, iterFunc func(rec *Record) bool)
	RawIterator(opts interface{}) interface{}
	NewBatch() interface{}
}

// Functions describes the functions of a storage engine
type Functions interface {
	TxCommitDiscarder
	Operations
	TxRenewer
}
