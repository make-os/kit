package client

import "github.com/make-os/lobe/api/types"

// PushKey provides access to the pushkey-related RPC methods
type PushKey interface {
	// GetOwner gets the account that owns the given push key
	GetOwner(addr string, blockHeight ...uint64) (*types.GetAccountResponse, error)

	// Register registers a public key as a push key
	Register(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error)
}

// Chain provides access to the chain-related RPC methods
type Chain interface {
	// Get gets the account that owns a push key
	Get(id string, blockHeight ...uint64) (*types.GetAccountResponse, error)
}

// Repo provides access to the repo-related RPC methods
type Repo interface {
	// Create creates a new repository
	Create(body *types.CreateRepoBody) (*types.CreateRepoResponse, error)

	// Get finds and returns a repository
	Get(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error)

	// AddContributors creates transaction to create a add repo contributors
	AddContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error)

	// VoteProposal creates transaction to vote for/against a repository's proposal
	VoteProposal(body *types.RepoVoteBody) (*types.HashResponse, error)
}

// RPC provides access to the rpc server-related methods
type RPC interface {
	// GetMethods gets all methods supported by the RPC server
	GetMethods() (*types.GetMethodResponse, error)
}

// Tx provides access to the transaction-related RPC methods
type Tx interface {
	// Send sends a signed transaction payload to the mempool
	Send(data map[string]interface{}) (*types.HashResponse, error)

	// Get gets a transaction by its hash
	Get(hash string) (*types.GetTxResponse, error)
}

// User provides access to user-related RPC methods
type User interface {
	// Get gets an account corresponding to a given address
	Get(address string, blockHeight ...uint64) (*types.GetAccountResponse, error)

	// Send creates a new repository
	Send(body *types.SendCoinBody) (*types.HashResponse, error)
}
