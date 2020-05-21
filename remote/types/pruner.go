package types

// RepoPruner provides repository pruning functionality
type RepoPruner interface {

	// Start starts the pruner
	Start()

	// Schedule schedules a repository for pruning
	Schedule(repoName string)

	// Prune prunes a repository only if it has no transactions in the transaction
	// and push pool. If force is set to true, the repo will be pruned regardless of
	// the existence of transactions in the pools.
	Prune(repoName string, force bool) error

	// Stop stops the pruner
	Stop()
}
