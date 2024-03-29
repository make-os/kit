package api

import (
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/rpc"
	tickettypes "github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
)

// ResultHash contains the hash of a transaction request
type ResultHash struct {
	Hash string `json:"hash"`
}

// ResultTx contains the result for a request to get a transaction
type ResultTx struct {
	Data   map[string]interface{} `json:"data"`
	Status string                 `json:"status"`
}

// ResultAccountNonce is the result for a request to get an account's nonce.
type ResultAccountNonce struct {
	Nonce string `json:"nonce"`
}

// ResultAccount is the result for a request to get an account.
type ResultAccount struct {
	*state.Account `json:",flatten"`
}

// ResultValidatorInfo is the result for request to get a node's validator information
type ResultValidatorInfo struct {
	PublicKey         string `json:"pubkey"`
	Address           string `json:"address"`
	PrivateKey        string `json:"privkey"`
	TendermintAddress string `json:"tmAddr"`
}

// ResultBlock is the result for a request to get a block.
type ResultBlock struct {
	*core_types.ResultBlock `json:",flatten"`
}

// ResultBlockInfo is the result for a request to get summarized block info.
type ResultBlockInfo struct {
	*state.BlockInfo `json:",flatten"`
}

// ResultDHTProvider describes a DHT provider
type ResultDHTProvider struct {
	ID        string   `json:"id"`
	Addresses []string `json:"addresses"`
}

// ResultValidators is the result for a request to a get block validator
type ResultValidator struct {
	Address           string `json:"address"`
	PubKey            string `json:"pubkey"`
	TicketID          string `json:"ticketId"`
	TendermintAddress string `json:"tmAddr"`
}

// ResultPushKey is the result for a request to get a push key.
type ResultPushKey struct {
	*state.PushKey `json:",flatten"`
}

// ResultCreateRepo is the result for a request to create a repository
type ResultCreateRepo struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

// BodyCreateRepo contains arguments for creating a repository
type BodyCreateRepo struct {
	Name        string
	Description string
	Nonce       uint64
	Value       float64
	Fee         float64
	Config      map[string]interface{}
	SigningKey  *ed25519.Key
}

// ResultRepository is the result for a request to get a repository
type ResultRepository struct {
	*state.Repository `json:",flatten"`
}

// GetRepoOpts contains arguments for fetching a repository
type GetRepoOpts struct {
	Height      uint64 `json:"height"`
	NoProposals bool   `json:"noProposals"`
}

// BodyRepoVote contains arguments for voting on a proposal
type BodyRepoVote struct {
	RepoName   string
	ProposalID string
	Vote       int
	Fee        float64
	Nonce      uint64
	SigningKey *ed25519.Key
}

// BodyRegisterPushKey contains arguments for registering a push key
type BodyRegisterPushKey struct {
	Nonce      uint64
	Fee        float64
	PublicKey  ed25519.PublicKey
	Scopes     []string
	FeeCap     float64
	SigningKey *ed25519.Key
}

// ResultRegisterPushKey is the result for a request to register a push key
type ResultRegisterPushKey struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

// BodyAddRepoContribs contains arguments for adding repo contributors
type BodyAddRepoContribs struct {
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
	SigningKey    *ed25519.Key
}

// ResultGetMethod is the response for RPC server methods
type ResultGetMethod struct {
	Methods []rpc.MethodInfo
}

// BodySendCoin contains arguments for sending coins
type BodySendCoin struct {
	Nonce      uint64
	Value      float64
	Fee        float64
	To         identifier.Address
	SigningKey *ed25519.Key
}

// BodySetCommission contains arguments for updating a validators commission value
type BodySetCommission struct {
	Nonce      uint64
	Fee        float64
	Commission float64
	SigningKey *ed25519.Key
}

// BodyBuyTicket contains arguments for purchasing a ticket
type BodyBuyTicket struct {
	Nonce      uint64
	Fee        float64
	Value      float64
	SigningKey *ed25519.Key
	Delegate   ed25519.PublicKey
	BLSPubKey  util.Bytes
}

// BodyTicketQuery contains arguments for querying tickets
type BodyTicketQuery struct {
	ProposerPubKey string
	QueryOption    *tickettypes.QueryOptions
}

// ResultTicket represents a ticket
type ResultTicket struct {
	*tickettypes.Ticket `json:",flatten"`
}

// ResultPoolSize describes size information of the mempool
type ResultPoolSize struct {
	Count int `json:"count"`
	Size  int `json:"size"`
}
