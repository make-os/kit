package client

import "github.com/make-os/lobe/api/types"

// PushKeyAPI provides access to the pushkey-related remote APIs.
type PushKey interface {
	// GetPushKeyOwnerNonce returns the nonce of the push key owner account
	GetOwnerNonce(pushKeyID string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error)

	// Get finds a push key by its ID.
	// If blockHeight is specified, only the block at the given height is searched.
	Get(pushKeyID string, blockHeight ...uint64) (*types.GetPushKeyResponse, error)

	// Register creates a transaction to register a push key
	Register(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error)
}

// Repo provides access to the repo-related remote APIs
type Repo interface {
	// Create creates transaction to create a new repository
	Create(body *types.CreateRepoBody) (*types.CreateRepoResponse, error)

	// VoteProposal creates transaction to vote for/against a repository's proposal
	VoteProposal(body *types.RepoVoteBody) (*types.HashResponse, error)

	// Get returns the repository corresponding to the given name
	Get(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error)

	// AddContributors creates transaction to create a add repo contributors
	AddContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error)
}

// TxAPI provides access to the transaction-related remote APIs.
type Tx interface {
	// Send sends a signed transaction to the mempool
	Send(data map[string]interface{}) (*types.HashResponse, error)

	// Get gets a transaction by hash
	Get(hash string) (*types.GetTxResponse, error)
}

// UserAPI provides access to user-related remote APIs.
type User interface {
	// GetNonce returns the nonce of an account
	GetNonce(address string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error)

	// Get returns the account corresponding to the given address
	Get(address string, blockHeight ...uint64) (*types.GetAccountResponse, error)

	// Send creates transaction to send coins to another user or a repository.
	Send(body *types.SendCoinBody) (*types.HashResponse, error)
}
