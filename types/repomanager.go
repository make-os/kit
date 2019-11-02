package types

import "context"

// RepoManager describes an interface for servicing
// and managing git repositories
type RepoManager interface {

	// Start starts the server
	Start()

	// Wait can be used by the caller to wait
	// till the server terminates
	Wait()

	// Stop shutsdown the server
	Stop(ctx context.Context)

	// CreateRepository creates a local git repository
	CreateRepository(name string) error
}
