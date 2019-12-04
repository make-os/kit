package logic

import (
	"fmt"

	"github.com/makeos/mosdef/validators"

	"github.com/makeos/mosdef/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// Transaction implements types.TxLogic. Provides functionalities for executing transactions.
type Transaction struct {
	logic types.Logic
}

// PrepareExec decodes the transaction from the abci request,
// performs final validation before executing the transaction.
// chainHeight: The height of the block chain
func (t *Transaction) PrepareExec(req abcitypes.RequestDeliverTx, chainHeight uint64) abcitypes.ResponseDeliverTx {

	// Decode tx bytes to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeFailedDecode,
			Log:  "failed to decode transaction from bytes",
		}
	}

	// Validate the transaction
	if err = validators.ValidateTx(tx, -1, t.logic); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeFailedDecode,
			Log:  "tx failed validation: " + err.Error(),
		}
	}

	// Execute the transaction
	if err = t.Exec(tx, chainHeight); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeExecFailure,
			Log:  "failed to execute tx: " + err.Error(),
		}
	}

	return abcitypes.ResponseDeliverTx{Code: 0}
}

// Exec execute a transaction that modifies the state.
// It returns error if the transaction is unknown.
// tx: The transaction to be processed
// chainHeight: The height of the block chain
func (t *Transaction) Exec(tx *types.Transaction, chainHeight uint64) error {
	spk := tx.SenderPubKey
	switch tx.Type {
	case types.TxTypeCoinTransfer:
		return t.execCoinTransfer(spk, tx.To, tx.Value, tx.Fee, chainHeight)
	case types.TxTypeValidatorTicket:
		return t.execValidatorStake(spk, tx.Value, tx.Fee, chainHeight)
	case types.TxTypeStorerTicket:
		return t.execStorerStake(spk, tx.Value, tx.Fee, chainHeight)
	case types.TxTypeSetDelegatorCommission:
		return t.execSetDelegatorCommission(spk, tx.Value, tx.Fee, chainHeight)
	case types.TxTypeUnbondStorerTicket:
		return t.execUnbond(tx.UnbondTicket.TicketID, spk, tx.Fee, chainHeight)
	case types.TxTypeRepoCreate:
		return t.execRepoCreate(spk, tx.RepoCreate.Name, tx.Fee, chainHeight)
	case types.TxTypeAddGPGPubKey:
		return t.execAddGPGKey(tx.GetGPGPublicKey(), spk, tx.Fee, chainHeight)
	case types.TxTypeEpochSecret:
		return nil
	default:
		return fmt.Errorf("unknown transaction type")
	}
}
