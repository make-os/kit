package types

import (
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/crypto/rand"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/util"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// BlockValidators contains validators of a block
type BlockValidators map[string]*Validator

// BlockInfo describes information about a block
type BlockInfo struct {
	AppHash             []byte `json:"appHash"`
	LastAppHash         []byte `json:"lastAppHash"`
	Hash                []byte `json:"hash"`
	Height              int64  `json:"height"`
	ProposerAddress     string `json:"proposerAddress"`
	EpochSecret         []byte `json:"epochSecret"`
	EpochPreviousSecret []byte `json:"epochPreviousSecret"`
	EpochRound          uint64 `json:"epochRound"`
	InvalidEpochSecret  bool   `json:"invalidEpochSecret"`
}

// Validator represents a validator
type Validator struct {
	PubKey   HexBytes `json:"publicKey,omitempty" mapstructure:"publicKey"`
	Power    int64    `json:"power" mapstructure:"power"`
	TicketID string   `json:"ticketID" mapstructure:"ticketID"`
}

// SystemKeeper describes an interface for accessing system data
type SystemKeeper interface {
	// SaveBlockInfo saves a committed block information
	SaveBlockInfo(info *BlockInfo) error

	// GetLastBlockInfo returns information about the last committed block
	GetLastBlockInfo() (*BlockInfo, error)

	// GetBlockInfo returns block information at a given height
	GetBlockInfo(height int64) (*BlockInfo, error)

	// MarkAsMatured sets the network maturity flag to true
	MarkAsMatured(maturityHeight uint64) error

	// GetNetMaturityHeight returns the height at which network maturity was attained
	GetNetMaturityHeight() (uint64, error)

	// IsMarkedAsMature returns true if the network has been flagged as mature.
	IsMarkedAsMature() (bool, error)

	// SetHighestDrandRound sets the highest drand round to r
	// only if r is greater than the current highest round.
	SetHighestDrandRound(r uint64) error

	// GetHighestDrandRound returns the highest drand round
	// known to the application
	GetHighestDrandRound() (uint64, error)

	// GetSecrets fetch secrets from blocks starting from a given
	// height back to genesis block. The argument limit puts a
	// cap on the number of secrets to be collected. If limit is
	// set to 0 or negative number, no limit is applied.
	// The argument skip controls how many blocks are skipped.
	// Skip is 1 by default. Blocks with an invalid secret or
	// no secret are ignored.
	GetSecrets(from, limit, skip int64) ([][]byte, error)
}

// TxKeeper describes an interface for managing transaction data
type TxKeeper interface {

	// Index takes a transaction and stores it.
	// It uses the tx hash as the index key
	Index(tx Tx) error

	// GetTx gets a transaction by its hash
	GetTx(hash []byte) (Tx, error)
}

// AccountKeeper describes an interface for accessing accounts
type AccountKeeper interface {
	// GetAccount returns an account by address.
	// It returns an empty Account if no account is found.
	// If block number is specified and greater than 0,
	// the account state at the given block number is
	// returned, otherwise the latest is returned
	GetAccount(address util.String, blockNum ...int64) *Account

	// Update resets an account to a new value
	Update(address util.String, upd *Account)
}

// AtomicLogic is like Logic but allows all operations
// performed to be atomically committed. The implementer
// must maintain a tx that all logical operations use and
// allow the tx to be committed or discarded
type AtomicLogic interface {
	Logic

	// Commit the underlying transaction.
	// Panics if called when no active transaction.
	Commit() error

	// Discard the underlying transaction
	// Panics if called when no active transaction.
	Discard()
}

// Logic provides an interface that allows
// access and modification to the state of the blockchain.
type Logic interface {
	Keepers

	// Tx returns the transaction logic
	Tx() TxLogic

	// SysLogic returns the app logic
	Sys() SysLogic

	// Validator returns the validator logic
	Validator() ValidatorLogic

	// DB returns the application's database
	DB() storage.Engine

	// StateTree manages the app state tree
	StateTree() Tree

	// WriteGenesisState initializes the app state with initial data
	WriteGenesisState() error

	// SetTicketManager sets the ticket manager
	SetTicketManager(tm TicketManager)

	// GetDRand returns a drand client
	GetDRand() rand.DRander

	// NewWithTx creates a new Logic instance but updates the keepers
	// with a new database transaction for atomic operations.
	// autoFinish: Commits/Discards the transaction after every operation
	// renew: Renews the transaction after every operation (only works if
	// autoFinish is true)
	// NewWithTx(autoFinish, renew bool) AtomicLogic
}

// Keepers describes modules for accessing the state and storage
// of various application components
type Keepers interface {

	// SysKeeper manages system state
	SysKeeper() SystemKeeper

	// AccountKeeper manages account state
	AccountKeeper() AccountKeeper

	// ValidatorKeeper returns the validator keeper
	ValidatorKeeper() ValidatorKeeper

	// TxKeeper returns the transaction keeper
	TxKeeper() TxKeeper

	// GetTicketManager returns the ticket manager
	GetTicketManager() TicketManager
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

// TxLogic provides an interface for executing transactions
type TxLogic interface {
	LogicCommon

	// PrepareExec decodes the transaction from the abci request,
	// performs final validation before executing the transaction.
	// chainHeight: The height of the block chain
	PrepareExec(req abcitypes.RequestDeliverTx, chainHeight uint64) abcitypes.ResponseDeliverTx

	// Exec execute a transaction that modifies the state.
	// It returns error if the transaction is unknown.
	// tx: The transaction to be processed
	// chainHeight: The height of the block chain
	Exec(tx *Transaction, chainHeight uint64) error

	// CanExecCoinTransfer checks whether the sender can transfer the value
	// and fee of the transaction based on the current state of their
	// account. It also ensures that the transaction's nonce is the
	// next/expected nonce value.
	// chainHeight: The height of the block chain
	CanExecCoinTransfer(txType int, senderPubKey *crypto.PubKey, recipientAddr,
		value, fee util.String, nonce, chainHeight uint64) error
}

// SysLogic provides an interface for managing system/app information
type SysLogic interface {
	LogicCommon

	// GetCurValidatorTicketPrice returns the current
	// price for a validator ticket
	GetCurValidatorTicketPrice() float64

	// CheckSetNetMaturity checks whether the network
	// has reached a matured period. If it has not,
	// we return error. However, if it is just
	// met the maturity condition in this call, we
	// mark the network as mature
	CheckSetNetMaturity() error

	// GetEpoch return the current and next epoch
	GetEpoch(curBlockHeight uint64) (int, int)

	// GetCurretEpochSecretTx returns an TxTypeEpochSecret transaction
	// only if the next block is the last block in the current epoch.
	GetCurretEpochSecretTx() (Tx, error)

	// MakeSecret generates a 64 bytes secret for validator
	// selection by xoring the last 32 valid epoch secrets.
	// The most recent secrets will be selected starting from
	// the given height down to genesis.
	// It returns ErrNoSecretFound if no error was found
	MakeSecret(height int64) ([]byte, error)
}
