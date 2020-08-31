package core

import (
	"encoding/json"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	tickettypes "github.com/make-os/lobe/ticket/types"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// BlockValidators contains validators of a block
type BlockValidators map[util.Bytes32]*Validator

// BlockInfo describes information about a block
type BlockInfo struct {
	AppHash         []byte     `json:"appHash"`
	LastAppHash     []byte     `json:"lastAppHash"`
	Hash            []byte     `json:"hash"`
	Height          util.Int64 `json:"height"`
	ProposerAddress []byte     `json:"proposerAddress"`
	Time            util.Int64 `json:"time"`
}

// Validator represents a validator
type Validator struct {
	PubKey   util.Bytes32  `json:"publicKey,omitempty" mapstructure:"publicKey"`
	TicketID util.HexBytes `json:"ticketID,omitempty" mapstructure:"ticketID"`
}

// SystemKeeper describes an interface for accessing system data
type SystemKeeper interface {

	// SaveBlockInfo saves a committed block information
	SaveBlockInfo(info *BlockInfo) error

	// GetLastBlockInfo returns information about the last committed block
	GetLastBlockInfo() (*BlockInfo, error)

	// GetBlockInfo returns block information at a given height
	GetBlockInfo(height int64) (*BlockInfo, error)

	// SetLastRepoObjectsSyncHeight sets the last block that was processed by the repo
	// object synchronizer
	SetLastRepoObjectsSyncHeight(height uint64) error

	// GetLastRepoObjectsSyncHeight returns the last block that was processed by the
	// repo object synchronizer
	GetLastRepoObjectsSyncHeight() (uint64, error)

	// SetHelmRepo sets the governing repository of the network
	SetHelmRepo(name string) error

	// GetHelmRepo gets the governing repository of the network
	GetHelmRepo() (string, error)
}

// TxKeeper describes an interface for managing transaction data
type TxKeeper interface {

	// Index takes a transaction and stores it.
	// It uses the tx hash as the index key
	Index(tx types.BaseTx) error

	// GetTx gets a transaction by its hash
	GetTx(hash []byte) (types.BaseTx, error)
}

// BalanceAccount represents an account that maintains currency balance
type BalanceAccount interface {
	GetBalance() util.String
	SetBalance(bal string)
	Clean(chainHeight uint64)
}

// AccountKeeper describes an interface for accessing account data
type AccountKeeper interface {
	// Get returns an account by address.
	//
	// ARGS:
	// address: The address of the account
	// blockNum: The target block to query (Optional. Default: latest)
	//
	// CONTRACT: It returns an empty Account if no account is found.
	Get(address identifier.Address, blockNum ...uint64) *state.Account

	// Update sets a new object at the given address.
	//
	// ARGS:
	// address: The address of the account to update
	// udp: The updated account object to replace the existing object.
	Update(address identifier.Address, upd *state.Account)
}

// TrackedRepo stores status info about a tracked repository or
type TrackedRepo struct {
	LastUpdated util.UInt64 `json:"lastUpdated" msgpack:"lastUpdated"`
}

// TrackedRepoKeeper describes an interface for managing tracked repositories.
type TrackedRepoKeeper interface {
	Add(targets string, height ...uint64) error
	Tracked() (res map[string]*TrackedRepo)
	Get(name string) *TrackedRepo
	Remove(targets string) error
}

// RepoKeeper describes an interface for accessing repository data
type RepoKeeper interface {
	// Get finds a repository by name.
	//
	// It will populate the proposals in the repo with their correct config
	// source from the version the repo that they where first appeared in.
	//
	// ARGS:
	// name: The name of the repository to find.
	// blockNum: The target block to query (Optional. Default: latest)
	//
	// CONTRACT: It returns an empty Repository if no repo is found.
	Get(name string, blockNum ...uint64) *state.Repository

	// GetNoPopulate fetches a repository by the given name without making additional
	// queries to populate the repo with associated objects.
	//
	// ARGS:
	// name: The name of the repository to find.
	// blockNum: The target block to query (Optional. Default: latest)
	//
	// CONTRACT: It returns an empty Repository if no repo is found.
	GetNoPopulate(name string, blockNum ...uint64) *state.Repository

	// Update sets a new object at the given name.
	//
	// ARGS:
	// name: The name of the repository to update
	// udp: The updated repository object to replace the existing object.
	Update(name string, upd *state.Repository)

	// IndexProposalVote indexes a proposal vote.
	//
	// ARGS:
	// name: The name of the repository
	// propID: The target proposal
	// voterAddr: The address of the voter
	// vote: Indicates the vote choice
	IndexProposalVote(name, propID, voterAddr string, vote int) error

	// GetProposalVote returns the vote choice of the
	// given voter for the given proposal
	//
	// ARGS:
	// name: The name of the repository
	// propID: The target proposal
	// voterAddr: The address of the voter
	GetProposalVote(name, propID, voterAddr string) (vote int, found bool, err error)

	// IndexProposalEnd indexes a proposal by its end height so it can be
	// tracked and finalized at the given height
	//
	// ARGS:
	// name: The name of the repository
	// propID: The target proposal
	// endHeight: The chain height when the proposal will stop accepting votes.
	IndexProposalEnd(name, propID string, endHeight uint64) error

	// GetProposalsEndingAt finds repo proposals ending at the given height
	//
	// ARGS:
	// height: The chain height when the proposal will stop accepting votes.
	GetProposalsEndingAt(height uint64) []*EndingProposals

	// MarkProposalAsClosed makes a proposal as "closed"
	//
	// ARGS:
	// name: The name of the repository
	// propID: The target proposal
	MarkProposalAsClosed(name, propID string) error

	// IsProposalClosed checks whether a proposal has been marked "closed"
	//
	// ARGS:
	// name: The name of the repository
	// propID: The target proposal
	IsProposalClosed(name, propID string) (bool, error)
}

// EndingProposals describes a proposal ending height
type EndingProposals struct {
	RepoName   string
	ProposalID string
	EndHeight  uint64
}

// NamespaceKeeper describes an interface for accessing namespace data
type NamespaceKeeper interface {
	// Get finds a namespace by name.
	// ARGS:
	// name: The name of the namespace to find.
	// blockNum: The target block to query (Optional. Default: latest)
	//
	// CONTRACT: It returns an empty Namespace if no matching namespace is found.
	Get(name string, blockNum ...uint64) *state.Namespace

	// GetTarget looks up the target of a full namespace path
	// ARGS:
	// path: The path to look up.
	// blockNum: The target block to query (Optional. Default: latest)
	GetTarget(path string, blockNum ...uint64) (string, error)

	// Update sets a new object at the given name.
	// ARGS:
	// name: The name of the namespace to update
	// udp: The updated namespace object to replace the existing object.
	Update(name string, upd *state.Namespace)
}

// PushKeyKeeper describes an interface for accessing push public key information
type PushKeyKeeper interface {

	// Update sets a new value for the given public key id
	//
	// ARGS:
	// pushKeyID: The public key unique ID
	// udp: The updated object to replace the existing object.
	Update(pushKeyID string, upd *state.PushKey) error

	// Get finds and returns a push key
	//
	// ARGS:
	// pushKeyID: The unique ID of the public key
	// blockNum: The target block to query (Optional. Default: latest)
	//
	// CONTRACT: It returns an empty Account if no account is found.
	Get(pushKeyID string, blockNum ...uint64) *state.PushKey

	// GetByAddress returns all public keys associated with the given address
	//
	// ARGS:
	// address: The target address
	GetByAddress(address string) (pushKeys []string)

	// Remove removes a push key by id
	//
	// ARGS:
	// pushKeyID: The public key unique ID
	Remove(pushKeyID string) bool
}

// AtomicLogic is like Logic but allows all operations
// performed to be atomically committed. The implementer
// must maintain a tx that all logical operations use and
// allow the tx to be committed or discarded
type AtomicLogic interface {
	Logic

	// GetDBTx returns the db transaction used by the logic and keepers
	GetDBTx() storage.Tx

	// Commit the state tree, database transaction and other
	// processes that needs to be finalized after a new tree
	// version is saved.
	// NOTE: The operations are not all atomic.
	Commit() error

	// Discard the underlying transaction
	// Panics if called when no active transaction.
	Discard()
}

// ValidateTxFunc represents a function for validating a transaction
type ValidateTxFunc func(tx types.BaseTx, i int, logic Logic) error

type ExecArgs struct {
	Tx             types.BaseTx
	ChainHeight    uint64
	ValidateTx     ValidateTxFunc
	SystemContract []SystemContract
}

// Logic provides an interface that allows
// access and modification to the state of the blockchain.
type Logic interface {
	Keepers

	// Validator returns the validator logic
	Validator() ValidatorLogic

	// DB returns the application's database
	DB() storage.Engine

	// StateTree manages the app state tree
	StateTree() tree.Tree

	// WriteGenesisState initializes the app state with initial data
	ApplyGenesisState(state json.RawMessage) error

	// SetTicketManager sets the ticket manager
	SetTicketManager(tm tickettypes.TicketManager)

	// SetRemoteServer sets the repository server manager
	SetRemoteServer(m RemoteServer)

	// GetRemoteServer returns the repository server manager
	GetRemoteServer() RemoteServer

	// DrySend checks whether the given sender can execute the transaction
	//
	// sender can be an address, identifier.Address or *crypto.PubKey
	DrySend(sender interface{}, value, fee util.String, nonce, chainHeight uint64) error

	// ExecTx executes a transaction.
	// chainHeight: The height of the block chain
	ExecTx(args *ExecArgs) abcitypes.ResponseDeliverTx

	// Cfg returns the application config
	Config() *config.AppConfig

	// GetMempoolReactor returns the mempool reactor
	GetMempoolReactor() MempoolReactor

	// SetMempoolReactor sets the mempool reactor
	SetMempoolReactor(mr MempoolReactor)

	// OnEndBlock is called within the ABCI EndBlock method;
	// Do things that need to happen after each block transactions are processed.
	OnEndBlock(block *BlockInfo) error
}

// Keepers describes modules for accessing the state and storage
// of various application components
type Keepers interface {

	// SysKeeper provides access to system or operation information.
	SysKeeper() SystemKeeper

	// TrackedRepoKeeper returns the track list keeper
	TrackedRepoKeeper() TrackedRepoKeeper

	// AccountKeeper manages and provides access to network accounts
	AccountKeeper() AccountKeeper

	// ValidatorKeeper manages and provides access to validators information
	ValidatorKeeper() ValidatorKeeper

	// TxKeeper manages and provides access to transaction information
	TxKeeper() TxKeeper

	// RepoKeeper manages and provides access to repository information
	RepoKeeper() RepoKeeper

	// PushKeyKeeper manages and provides access to registered push keys
	PushKeyKeeper() PushKeyKeeper

	// GetTicketManager manages and provides access to ticket information
	GetTicketManager() tickettypes.TicketManager

	// NamespaceKeeper manages and provides access to namespace information
	NamespaceKeeper() NamespaceKeeper
}

// LogicCommon describes a common functionalities for
// all logic providers
type LogicCommon interface{}

// ValidatorKeeper describes an interface for managing validator information
type ValidatorKeeper interface {

	// GetByHeight gets validators at the given height. If height is <= 0, the
	// validator set of the highest height is returned.
	GetByHeight(height int64) (BlockValidators, error)

	// Index adds a set of validators associated to the given height
	Index(height int64, validators []*Validator) error
}

// ValidatorLogic provides functionalities for managing
// and deriving validators.
type ValidatorLogic interface {
	LogicCommon

	// Index indexes the validator set for the given height.
	Index(height int64, valUpdates []abcitypes.ValidatorUpdate) error
}

// SystemContract represents a system contract
type SystemContract interface {

	// Init initializes the contract
	// logic is the logic manager
	// tx is the transaction to execute.
	// curChainHeight is the current height of the chain
	Init(Keepers, types.BaseTx, uint64) SystemContract

	// CanExec checks whether the given tx type can be executed by the contract.
	CanExec(tx types.TxCode) bool

	// Exec executes the transaction
	Exec() error
}

// ProposalApplyArgs contains arguments passed to a proposal contract Apply function
type ProposalApplyArgs struct {
	Proposal    state.Proposal
	Repo        *state.Repository
	Keepers     Keepers
	ChainHeight uint64
}

// ProposalContract represents a system contract that is able to execute proposal transactions
// and apply proposal changes to the world state.
type ProposalContract interface {
	SystemContract

	// Apply is called when the proposal needs to be applied to the state.
	Apply(args *ProposalApplyArgs) error
}
