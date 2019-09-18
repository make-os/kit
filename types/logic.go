package types

import (
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// Logic provides an interface that allows
// access and modification to the state of the blockchain.
type Logic interface {
	Tx() TxLogic
	DB() storage.Engine
	StateTree() *tree.SafeTree
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
