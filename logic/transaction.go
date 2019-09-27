package logic

import (
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
// CONTRACT: Expects req.Tx to be the raw transaction bytes
func (t *Transaction) PrepareExec(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Decode tx bytes to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: ErrCodeFailedDecode,
			Log:  "failed to decode transaction from bytes",
		}
	}

	// Validate the transaction
	if err = validators.ValidateTx(tx, -1, t.logic); err != nil {
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
		return t.transferTo(tx.SenderPubKey, tx.To, tx.Value, tx.Fee, tx.GetNonce())
	default:
		return fmt.Errorf("unknown transaction type")
	}
}

// CanTransferCoin checks whether the sender can transfer
// the value and fee of the transaction based on the
// current state of their account. It also ensures that the
// transaction's nonce is the next/expected nonce value.
func (t *Transaction) CanTransferCoin(senderPubKey, recipientAddr, value, fee util.String,
	nonce uint64) error {

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

	// Ensure the transaction nonce is the next expected nonce
	expectedNonce := senderAcct.Nonce + 1
	if expectedNonce != nonce {
		return fmt.Errorf("tx has invalid nonce (%d), expected (%d)", nonce, expectedNonce)
	}

	// Ensure sender has enough balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.Balance.Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return fmt.Errorf("sender's account balance is insufficient")
	}

	return nil
}

// transferTo transfer units of the native currency
// from a sender account to a recipient account
func (t *Transaction) transferTo(senderPubKey, recipientAddr, value, fee util.String,
	nonce uint64) error {

	if err := t.CanTransferCoin(senderPubKey, recipientAddr, value, fee, nonce); err != nil {
		return err
	}

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	recipientAcct := acctKeeper.GetAccount(recipientAddr)

	// Ensure sender has enough balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.Balance.Decimal()

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
