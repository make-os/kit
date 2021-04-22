package miner

type Miner interface {
	// Start starts the miner.
	//  - Returns ErrNodeSyncing if node is still syncing.
	//  - if scheduleStart is true and the node is still syncing, the start request
	//    will be re-attempted every minute until the node has fully synced.
	//  - if scheduleStart is true and the node is still syncing, nil is returned.
	Start(scheduleStart bool) error

	// IsMining checks if the miner is running
	IsMining() bool

	// Stop stops the miner
	Stop()

	// GetHashrate returns the moving average rate of hashing per second
	GetHashrate() float64
}
