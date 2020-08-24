package types

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util/identifier"
)

// HashResponse contains the hash of a transaction request
type HashResponse struct {
	Hash string `json:"hash"`
}

// GetTxResponse contains the response of a request to get a transaction
type GetTxResponse struct {
	Data   map[string]interface{} `json:"data"`
	Status string                 `json:"status"`
}

// GetAccountNonceResponse is the response of a request for an account's nonce.
type GetAccountNonceResponse struct {
	Nonce string `json:"nonce"`
}

// GetAccountResponse is the response of a request for an account.
type GetAccountResponse struct {
	*state.Account `json:",flatten"`
}

// GetAccountResponse is the response of a request for a push key.
type GetPushKeyResponse struct {
	*state.PushKey `json:",flatten"`
}

// CreateRepoResponse is the response of a request to create a repository
type CreateRepoResponse struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

// CreateRepoBody contains arguments for creating a repository
type CreateRepoBody struct {
	Name       string
	Nonce      uint64
	Value      float64
	Fee        float64
	Config     map[string]interface{}
	SigningKey *crypto.Key
}

// GetRepoResponse is the response of a request to get a repository
type GetRepoResponse struct {
	*state.Repository `json:",flatten"`
}

// GetRepoOpts contains arguments for fetching a repository
type GetRepoOpts struct {
	Height      uint64 `json:"height"`
	NoProposals bool   `json:"noProposals"`
}

// RepoVoteBody contains arguments for voting on a proposal
type RepoVoteBody struct {
	RepoName   string
	ProposalID string
	Vote       int
	Fee        float64
	Nonce      uint64
	SigningKey *crypto.Key
}

// RegisterPushKeyBody contains arguments for registering a push key
type RegisterPushKeyBody struct {
	Nonce      uint64
	Fee        float64
	PublicKey  crypto.PublicKey
	Scopes     []string
	FeeCap     float64
	SigningKey *crypto.Key
}

// RegisterPushKeyResponse is the response of a request to register a push key
type RegisterPushKeyResponse struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

// AddRepoContribsBody contains arguments for adding repo contributors
type AddRepoContribsBody struct {
	RepoName      string
	ProposalID    string
	PushKeys      []string
	FeeCap        float64
	FeeMode       int
	Nonce         uint64
	Value         float64
	Fee           float64
	Namespace     string
	NamespaceOnly string
	Policies      []*state.ContributorPolicy
	SigningKey    *crypto.Key
}

// GetMethodResponse is the response for RPC server methods
type GetMethodResponse struct {
	Methods []rpc.MethodInfo
}

// SendCoinBody contains arguments for sending coins
type SendCoinBody struct {
	Nonce      uint64
	Value      float64
	Fee        float64
	To         identifier.Address
	SigningKey *crypto.Key
}
