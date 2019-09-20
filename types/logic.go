package types

import (
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/util"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// BlockInfo describes information about a block
type BlockInfo struct {
	AppHash     []byte
	LastAppHash []byte
	Hash        []byte
	Height      int64
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
	Tx() TxLogic
	DB() storage.Engine
	StateTree() *tree.SafeTree
	SysKeeper() SystemKeeper
	AccountKeeper() AccountKeeper
	WriteGenesisState() error
}

// LogicCommon describes a common functionalities for
// all logic providers
type LogicCommon interface {
}

// TxLogic provides an interface for executing transactions
type TxLogic interface {
	LogicCommon
	PrepareExec(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx
	Exec(tx *Transaction) error
}
