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
	tx, err := types.DecodeTx(req.Tx)
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
func (t *Transaction) Exec(tx types.BaseTx, chainHeight uint64) error {
	spk := tx.GetSenderPubKey()

	switch o := tx.(type) {
	case *types.TxCoinTransfer:
		return t.execCoinTransfer(spk, o.To, o.Value, o.Fee, chainHeight)

	case *types.TxTicketPurchase:
		switch o.GetType() {
		case types.TxTypeValidatorTicket:
			return t.execValidatorStake(spk, o.Value, o.Fee, chainHeight)
		case types.TxTypeStorerTicket:
			return t.execStorerStake(spk, o.Value, o.Fee, chainHeight)
		default:
			return fmt.Errorf("unknown transaction type")
		}

	case *types.TxSetDelegateCommission:
		return t.execSetDelegatorCommission(spk, o.Commission, o.Fee, chainHeight)

	case *types.TxTicketUnbond:
		return t.execUnbond([]byte(o.TicketHash), spk, o.Fee, chainHeight)

	case *types.TxRepoCreate:
		return t.execRepoCreate(spk, o.Name, o.Fee, chainHeight)

	case *types.TxAddGPGPubKey:
		return t.execAddGPGKey(o.PublicKey, spk, o.Fee, chainHeight)

	case *types.TxEpochSecret:
		return nil

	default:
		return fmt.Errorf("unknown transaction type")
	}
}
