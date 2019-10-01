package logic

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/validators"

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
	case types.TxTypeCoinTransfer:
		return t.transferCoin(tx.SenderPubKey, tx.To, tx.Value, tx.Fee, tx.GetNonce())
	case types.TxTypeTicketValidator:
		return t.stakeValidatorCoin(tx.SenderPubKey, tx.Value, tx.Fee, tx.GetNonce())
	default:
		return fmt.Errorf("unknown transaction type")
	}
}

// CanTransferCoin checks whether the sender can transfer
// the value and fee of the transaction based on the
// current state of their account. It also ensures that the
// transaction's nonce is the next/expected nonce value.
func (t *Transaction) CanTransferCoin(
	txType int,
	senderPubKey *crypto.PubKey,
	recipientAddr,
	value,
	fee util.String,
	nonce uint64) error {

	var err error

	// Ensure recipient address is valid.
	// Ignore for ticket purchases tx as a recipient address is not required.
	if txType != types.TxTypeTicketValidator {
		if err = crypto.IsValidAddr(recipientAddr.String()); err != nil {
			return fmt.Errorf("invalid recipient address: %s", err)
		}
	}

	// For validator ticket transaction:
	// The tx value must be equal or greater than the current ticket price.
	if txType == types.TxTypeTicketValidator {
		curTicketPrice := t.logic.Sys().GetCurTicketPrice()
		if value.Decimal().LessThan(decimal.NewFromFloat(curTicketPrice)) {
			return fmt.Errorf("sender's spendable account balance is insufficient to cover "+
				"ticket price (%f)", curTicketPrice)
		}
	}

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := senderPubKey.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Ensure the transaction nonce is the next expected nonce
	expectedNonce := senderAcct.Nonce + 1
	if expectedNonce != nonce {
		return fmt.Errorf("tx has invalid nonce (%d), expected (%d)", nonce, expectedNonce)
	}

	// Ensure sender has enough spendable balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.GetSpendableBalance().Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return fmt.Errorf("sender's spendable account balance is insufficient")
	}

	return nil
}

// stakeValidatorCoin moves a given amount from the spendable
// balance of an account to the validator stake balance.
func (t *Transaction) stakeValidatorCoin(
	senderPubKey,
	value,
	fee util.String,
	nonce uint64) error {

	spk, err := crypto.PubKeyFromBase58(senderPubKey.String())
	if err != nil {
		return fmt.Errorf("invalid sender public key: %s", err)
	}

	// Ensure the account has sufficient balance and nonce
	if err := t.CanTransferCoin(types.TxTypeTicketValidator, spk, "",
		value, fee, nonce); err != nil {
		return err
	}

	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Get the current validator stakes, add the new value to it
	// and update the account
	curValStake := senderAcct.Stakes.Get(types.StakeNameValidator)
	newValStake := curValStake.Decimal().Add(value.Decimal()).String()
	senderAcct.Stakes.Add(types.StakeNameValidator, util.String(newValStake))

	// Deduct the transaction fee and increment nonce
	senderBal := senderAcct.Balance.Decimal()
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	return nil
}

// transferCoin transfer units of the native currency
// from a sender account to another account
func (t *Transaction) transferCoin(
	senderPubKey,
	recipientAddr,
	value,
	fee util.String,
	nonce uint64) error {

	spk, err := crypto.PubKeyFromBase58(senderPubKey.String())
	if err != nil {
		return fmt.Errorf("invalid sender public key: %s", err)
	}

	// Ensure the account has sufficient balance and nonce
	if err := t.CanTransferCoin(types.TxTypeCoinTransfer, spk, recipientAddr,
		value, fee, nonce); err != nil {
		return err
	}

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	recipientAcct := acctKeeper.GetAccount(recipientAddr)
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
