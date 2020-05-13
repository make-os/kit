package logic

import (
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/state"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/logic/contracts"
	"gitlab.com/makeos/mosdef/types/core"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/makeos/mosdef/types"
)

// ExecTx executes a transaction.
// chainHeight: The height of the block chain
func (l *Logic) ExecTx(args *core.ExecArgs) abcitypes.ResponseDeliverTx {

	var err error
	var errCode = types.ErrCodeExecFailure

	// Validate the transaction
	if err = args.ValidateTx(args.Tx, -1, l); err != nil {
		return abcitypes.ResponseDeliverTx{Code: types.ErrCodeFailedDecode, Log: "tx failed validation: " + err.Error()}
	}

	sysContracts := args.SystemContract
	if len(sysContracts) == 0 {
		sysContracts = contracts.SystemContracts
	}

	// Find a contract that can execute the transaction
	for _, contract := range sysContracts {
		if !contract.CanExec(args.Tx.GetType()) {
			continue
		}

		// Initialize the contract and execute the transaction
		if err := contract.Init(l, args.Tx, args.ChainHeight).Exec(); err != nil {
			if errors.Cause(err).Error() == dht.ErrObjNotFound.Error() {
				errCode = state.ErrCodeReExecBlock
			}
			return abcitypes.ResponseDeliverTx{Code: errCode, Log: "failed to execute tx: " + err.Error()}
		}

		return abcitypes.ResponseDeliverTx{Code: 0}
	}

	return abcitypes.ResponseDeliverTx{
		Code: errCode,
		Log:  "failed to execute tx: no executor found",
	}
}
