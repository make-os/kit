package types

import (
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/util"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// BlockInfo describes information about a block
type BlockInfo struct {
	AppHash         []byte
	LastAppHash     []byte
	Hash            []byte
	Height          int64
	ProposerAddress string
}

// SystemKeeper describes an interface for accessing system data
type SystemKeeper interface {
	SaveBlockInfo(info *BlockInfo) error
	GetLastBlockInfo() (*BlockInfo, error)
}

// AccountKeeper describes an interface for accessing accounts
type AccountKeeper interface {
	GetAccount(address util.String, blockNum ...int64) *Account
	Update(address util.String, upd *Account)
}

// Logic provides an interface that allows
// access and modification to the state of the blockchain.
type Logic interface {

	// Tx returns the transaction logic
	Tx() TxLogic

	// SysLogic returns the app logic
	Sys() SysLogic

	// DB returns the application's database
	DB() storage.Engine

	// StateTree manages the app state tree
	StateTree() Tree

	// SysKeeper manages system state
	SysKeeper() SystemKeeper

	// AccountKeeper manages account state
	AccountKeeper() AccountKeeper

	// WriteGenesisState initializes the app state with initial data
	WriteGenesisState() error
}

// LogicCommon describes a common functionalities for
// all logic providers
type LogicCommon interface{}

// TxLogic provides an interface for executing transactions
type TxLogic interface {
	LogicCommon
	PrepareExec(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx
	Exec(tx *Transaction) error
	CanTransferCoin(txType int, senderPubKey *crypto.PubKey, recipientAddr,
		value, fee util.String, nonce uint64) error
}

// SysLogic provides an interface for managing system/app information
type SysLogic interface {
	LogicCommon
	GetCurTicketPrice() float64
}
