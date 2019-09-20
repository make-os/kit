package logic

import (
	"encoding/hex"
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

const (
	// ErrCodeFailedDecode refers to a failed decoded operation
	ErrCodeFailedDecode = uint32(1)

	// ErrExecFailure refers to a failure in executing an state transition operation
	ErrExecFailure = 2
)

// Transaction implements types.TxLogic. Provides functionalities
// for executing transactions
type Transaction struct {
	logic types.Logic
}

// PrepareExec decodes the transaction from the abci request,
// performs final validation before executing the transaction.
func (t *Transaction) PrepareExec(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Decode tx from bytes
	hexDecode, err := hex.DecodeString(string(req.GetTx()))
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: ErrCodeFailedDecode,
			Log:  "failed to decode transaction from hex to bytes",
		}
	}

	// Decode tx bytes to Transaction
	tx, err := types.NewTxFromBytes(hexDecode)
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: ErrCodeFailedDecode,
			Log:  "failed to decode transaction from bytes",
		}
	}

	// Validate the transaction
	if err = validators.ValidateTx(tx, -1); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: ErrCodeFailedDecode,
			Log:  "tx failed validation: " + err.Error(),
		}
	}

	// Execute the transaction
	if err = t.Exec(tx); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: ErrExecFailure,
			Log:  "failed to execute tx: " + err.Error(),
		}
	}

	return abcitypes.ResponseDeliverTx{Code: 0}
}

// Exec execute a transaction that modifies the state.
// It returns error if the transaction is unknown.
func (t *Transaction) Exec(tx *types.Transaction) error {
	switch tx.Type {
	case types.TxTypeCoin:
		return t.transferTo(tx.SenderPubKey, tx.To, tx.Value, tx.Fee)
	default:
		return fmt.Errorf("unknown transaction type")
	}
}

// transferTo transfer units of the native currency
// from a sender account to a recipient account
func (t *Transaction) transferTo(senderPubKey, recipientAddr, value, fee util.String) error {

	spk, err := crypto.PubKeyFromBase58(senderPubKey.String())
	if err != nil {
		return fmt.Errorf("invalid sender public key: %s", err)
	}

	// Ensure recipient address is valid
	if err = crypto.IsValidAddr(recipientAddr.String()); err != nil {
		return fmt.Errorf("invalid recipient address: %s", err)
	}

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	recipientAcct := acctKeeper.GetAccount(recipientAddr)

	// Ensure sender has enough balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.Balance.Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return fmt.Errorf("sender's account balance is insufficient")
	}

	// Deduct the spend amount from the sender's account and increment nonce
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	// Update the recipient account
	recipientBal := recipientAcct.Balance.Decimal()
	recipientAcct.Balance = util.String(recipientBal.Add(value.Decimal()).String())
	acctKeeper.Update(recipientAddr, recipientAcct)

	return nil
}
