package types

// PushPool represents a pool for holding and ordering git push transactions
type PushPool interface {

	// Register a push transaction to the pool.
	//
	// Check all the references to ensure there are no identical (same repo,
	// reference and nonce) references with same nonce in the pool. A valid
	// reference is one which has no identical reference with a higher fee rate in
	// the pool. If an identical reference exist in the pool with an inferior fee
	// rate, the existing tx holding the reference is eligible for replacable by tx
	// holding the reference with a superior fee rate. In cases where more than one
	// reference of tx is superior to multiple references in multiple transactions,
	// replacement will only happen if the fee rate of tx is higher than the
	// combined fee rate of the replaceable transactions.
	//
	// noValidation disables tx validation
	Add(tx PushNotice, noValidation ...bool) error

	// Full returns true if the pool is full
	Full() bool

	// RepoHasPushNote returns true if the given repo has a transaction in the pool
	RepoHasPushNote(repo string) bool

	// Get finds and returns a push note
	Get(noteID string) *PushNote

	// Len returns the number of items in the pool
	Len() int

	// Remove removes a push note
	Remove(pushNote PushNotice)
}
