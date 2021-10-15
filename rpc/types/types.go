package types

import (
	"fmt"
	"strings"

	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util"
)

// PushKey provides access to the pushkey-related RPC methods
type PushKey interface {
	// GetOwner gets the account that owns the given push key
	GetOwner(addr string, blockHeight ...uint64) (*api.ResultAccount, error)

	// Register registers a public key as a push key
	Register(body *api.BodyRegisterPushKey) (*api.ResultRegisterPushKey, error)
}

// Client represents a JSON-RPC client
type Client interface {

	// GetOptions returns the client's option
	GetOptions() *Options

	// Call calls a method on the RPCClient service.
	Call(method string, params interface{}) (res util.Map, statusCode int, err error)

	// Node exposes methods for accessing chain information
	Node() Node

	// PushKey exposes methods for managing push keys
	PushKey() PushKey

	// Pool exposes methods for managing push keys
	Pool() Pool

	// Repo exposes methods for managing repositories
	Repo() Repo

	// RPC exposes methods for managing the RPC server
	RPC() RPC

	// Tx exposes methods for creating and accessing the transactions
	Tx() Tx

	// User exposes methods for accessing user information
	User() User

	// DHT exposes methods for accessing the DHT network
	DHT() DHT

	// Ticket exposes methods for purchasing and managing tickets
	Ticket() Ticket
}

// Node provides access to the chain-related RPC methods
type Node interface {
	// GetBlock gets a block by height
	GetBlock(height uint64) (*api.ResultBlock, error)

	// GetHeight returns the height of the blockchain
	GetHeight() (uint64, error)

	// GetBlockInfo gets a summarized block data for the given height
	GetBlockInfo(height uint64) (*api.ResultBlockInfo, error)

	// GetValidators gets validators at a given block height
	GetValidators(height uint64) ([]*api.ResultValidator, error)

	// IsSyncing checks whether the node is synchronizing with peers
	IsSyncing() (bool, error)
}

// DHT provides access to the DHT-related RPC methods
type DHT interface {
	// GetPeers returns node IDs of connected peers
	GetPeers() ([]string, error)

	// GetProviders returns providers of the given key
	GetProviders(key string) ([]*api.ResultDHTProvider, error)

	// Announce announces the given key to the network
	Announce(key string) error

	// GetRepoObjectProviders returns providers for the given repository object hash
	GetRepoObjectProviders(hash string) ([]*api.ResultDHTProvider, error)

	// Store stores a value under the given key on the DHT
	Store(key, value string) error

	// Lookup finds a value stored under the given key
	Lookup(key string) (string, error)
}

// Pool provides access to a nodes transaction and push pools
type Pool interface {

	// GetSize returns size information of the mempool
	GetSize() (*api.ResultPoolSize, error)

	// GetPushPoolSize returns size information of the mempool
	GetPushPoolSize() (int, error)
}

// Repo provides access to the repo-related RPC methods
type Repo interface {
	// Create creates a new repository
	Create(body *api.BodyCreateRepo) (*api.ResultCreateRepo, error)

	// Get finds and returns a repository
	Get(name string, opts ...*api.GetRepoOpts) (*api.ResultRepository, error)

	// AddContributors creates transaction to create a add repo contributors
	AddContributors(body *api.BodyAddRepoContribs) (*api.ResultHash, error)

	// VoteProposal creates transaction to vote for/against a repository's proposal
	VoteProposal(body *api.BodyRepoVote) (*api.ResultHash, error)
}

// RPC provides access to the rpc server-related methods
type RPC interface {
	// GetMethods gets all methods supported by the RPC server
	GetMethods() ([]rpc.MethodInfo, error)
}

// Tx provides access to the transaction-related RPC methods
type Tx interface {
	// Send sends a signed transaction payload to the mempool
	Send(data map[string]interface{}) (*api.ResultHash, error)

	// Get gets a transaction by its hash
	Get(hash string) (*api.ResultTx, error)
}

// User provides access to user-related RPC methods
type User interface {
	// Get gets an account corresponding to a given address
	Get(address string, blockHeight ...uint64) (*api.ResultAccount, error)

	// Send creates a new repository
	Send(body *api.BodySendCoin) (*api.ResultHash, error)

	// GetNonce gets the nonce of a user account corresponding to the given address
	GetNonce(address string, blockHeight ...uint64) (uint64, error)

	// GetKeys finds an account by address
	GetKeys() ([]string, error)

	// GetBalance returns the spendable balance of an account
	GetBalance(address string, blockHeight ...uint64) (float64, error)

	// GetStakedBalance returns the staked coin balance of an account
	GetStakedBalance(address string, blockHeight ...uint64) (float64, error)

	// GetValidator get the validator information of the node
	GetValidator(includePrivKey bool) (*api.ResultValidatorInfo, error)

	// GetPrivateKey returns the private key of a key on the keystore
	GetPrivateKey(address string, passphrase string) (string, error)

	// GetPublicKey returns the public key of a key on the keystore
	GetPublicKey(address string, passphrase string) (string, error)

	// SetCommission update the validator commission percentage of an account
	SetCommission(body *api.BodySetCommission) (*api.ResultHash, error)
}

// Ticket provides access to ticket-related RPC methods
type Ticket interface {

	// Buy creates a transaction to buy a validator ticket
	Buy(body *api.BodyBuyTicket) (*api.ResultHash, error)

	// BuyHost creates a transaction to buy a host ticket
	BuyHost(body *api.BodyBuyTicket) (*api.ResultHash, error)

	// List returns active validator tickets associated with a public key
	List(body *api.BodyTicketQuery) (res []*api.ResultTicket, err error)

	// ListHost returns active hosts tickets associated with a public key
	ListHost(body *api.BodyTicketQuery) (res []*api.ResultTicket, err error)
}

// Options describes the options used to configure the client
type Options struct {
	Host     string
	Port     int
	User     string
	Password string
}

// URL returns a fully formed url to use for making requests
func (o *Options) URL() string {
	host := o.Host
	if !strings.Contains(o.Host, "http") {
		host = "http://" + o.Host
	}
	if o.Port > 0 {
		host = host + ":" + fmt.Sprintf("%d", o.Port)
	}
	return host + "/rpc"
}
