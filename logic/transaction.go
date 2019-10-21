package logic

import (
	"fmt"

	"github.com/makeos/mosdef/params"

	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
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
	switch tx.Type {
	case types.TxTypeExecCoinTransfer:
		return t.execCoinTransfer(tx.SenderPubKey, tx.To, tx.Value, tx.Fee, tx.GetNonce(), chainHeight)
	case types.TxTypeGetTicket:
		return t.execValidatorStake(tx.SenderPubKey, tx.Value, tx.Fee, tx.GetNonce(), chainHeight)
	case types.TxTypeSetDelegatorCommission:
		return t.setDelegatorCommission(tx.SenderPubKey, tx.Value)
	case types.TxTypeEpochSecret:
		return nil
	default:
		return fmt.Errorf("unknown transaction type")
	}
}

// CanExecCoinTransfer checks whether the sender can transfer the value
// and fee of the transaction based on the current state of their
// account. It also ensures that the transaction's nonce is the
// next/expected nonce value.
// txType: The transaction type
// senderPubKey: The public key of the tx sender.
// recipientAddr: Recipient address
// value: The value of the transaction
// fee: The fee paid by the sender.
// nonce: The nonce of the transaction.
// chainHeight: The height of the block chain
func (t *Transaction) CanExecCoinTransfer(
	txType int,
	senderPubKey *crypto.PubKey,
	recipientAddr,
	value,
	fee util.String,
	nonce,
	chainHeight uint64) error {

	var err error

	// Ensure recipient address is valid.
	// Ignore for ticket purchases tx as a recipient address is not required.
	if txType != types.TxTypeGetTicket {
		if err = crypto.IsValidAddr(recipientAddr.String()); err != nil {
			return fmt.Errorf("invalid recipient address: %s", err)
		}
	}

	// For validator ticket transaction:
	// The tx value must be equal or greater than the current ticket price.
	// TODO: The error message is wrong. It is the transaction value that is
	// insufficient. Not the sender's account balance
	if txType == types.TxTypeGetTicket {
		curTicketPrice := t.logic.Sys().GetCurValidatorTicketPrice()
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
	senderBal := senderAcct.GetSpendableBalance(chainHeight).Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return fmt.Errorf("sender's spendable account balance is insufficient")
	}

	return nil
}

// execValidatorStake sets aside some balance as validator stake.
//
// senderPubKey: The public key of the tx sender.
// value: The value of the transaction.
// fee: The fee paid by the sender.
// nonce: The nonce of the transaction.
// chainHeight: The current chain height.
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execValidatorStake(
	senderPubKey,
	value,
	fee util.String,
	nonce,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Deduct the transaction fee and increment nonce
	senderBal := senderAcct.Balance.Decimal()
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Determine unbond height. The unbond height is height of the next block
	// (or proposed block) plus minimum ticket maturation duration, max ticket
	// active duration + thawing period.
	unbondHeight := chainHeight + 1 + uint64(params.MinTicketMatDur) +
		uint64(params.MaxTicketActiveDur) +
		uint64(params.NumBlocksInThawPeriod)

	// Add a stake entry
	senderAcct.Stakes.Add(types.StakeTypeValidator, value, unbondHeight)

	// Update the sender account
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}

// execCoinTransfer transfers units of the native currency from a sender account
// to another account.
// EXPECT: Syntactic and consistency validation to have been performed by caller.
//
// ARGS:
// senderPubKey: The sender's public key
// recipientAddr: The recipient address
// value: The value of the transaction
// fee: The transaction fee
// nonce: The transaction nonce
// chainHeight: The current chain height.
func (t *Transaction) execCoinTransfer(
	senderPubKey,
	recipientAddr,
	value,
	fee util.String,
	nonce,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender account and balance
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	senderBal := senderAcct.GetSpendableBalance(chainHeight).Decimal()

	// Deduct the spend amount from the sender's account and increment nonce
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up unbonded stakes and update sender account
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	// Get recipient account only if recipient and sender are different,
	// otherwise use the sender account as recipient account
	var recipientAcct = senderAcct
	if !sender.Equal(recipientAddr) {
		recipientAcct = acctKeeper.GetAccount(recipientAddr)
	}

	// Add the transaction value to the recipient balance
	recipientBal := recipientAcct.GetSpendableBalance(chainHeight).Decimal()
	recipientAcct.Balance = util.String(recipientBal.Add(value.Decimal()).String())

	// Clean up unbonded stakes and update recipient account
	recipientAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(recipientAddr, recipientAcct)

	return nil
}

// setDelegatorCommission sets the delegator commission of an account
//
// ARGS:
// senderPubKey: The sender's public key
// value: The target commission (in percentage)
func (t *Transaction) setDelegatorCommission(senderPubKey, value util.String) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)

	// Set the new commission and increment nonce
	senderAcct.DelegatorCommission, _ = value.Decimal().Float64()
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	return nil
}
