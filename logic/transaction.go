package logic

import (
	"fmt"

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
//
// ARGS:
// txType: The transaction type
// senderPubKey: The public key of the tx sender.
// value: The value of the transaction
// fee: The fee paid by the sender.
// chainHeight: The height of the block chain
func (t *Transaction) CanExecCoinTransfer(
	txType int,
	senderPubKey *crypto.PubKey,
	value,
	fee util.String,
	nonce,
	chainHeight uint64) error {

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	sender := senderPubKey.Addr()
	senderAcct := acctKeeper.GetAccount(sender, int64(chainHeight))

	// Ensure the transaction nonce is the next expected nonce
	expectedNonce := senderAcct.Nonce + 1
	if expectedNonce != nonce {
		return types.FieldError("value", fmt.Sprintf("tx has invalid nonce (%d), expected (%d)",
			nonce, expectedNonce))
	}

	// Ensure sender has enough spendable balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.GetSpendableBalance(chainHeight).Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return types.FieldError("value", fmt.Sprintf("sender's spendable account "+
			"balance is insufficient"))
	}

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
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execCoinTransfer(
	senderPubKey,
	recipientAddr,
	value util.String,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender account and balance
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender, int64(chainHeight))
	senderBal := senderAcct.Balance.Decimal()

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
		recipientAcct = acctKeeper.GetAccount(recipientAddr, int64(chainHeight))
	}

	// Add the transaction value to the recipient balance
	recipientBal := recipientAcct.Balance.Decimal()
	recipientAcct.Balance = util.String(recipientBal.Add(value.Decimal()).String())

	// Clean up unbonded stakes and update recipient account
	recipientAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(recipientAddr, recipientAcct)

	return nil
}

// execSetDelegatorCommission sets the delegator commission of an account
//
// ARGS:
// senderPubKey: The sender's public key
// value: The target commission (in percentage)
// fee: The fee paid by the sender
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execSetDelegatorCommission(
	senderPubKey,
	value,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender, int64(chainHeight))
	senderBal := senderAcct.Balance.Decimal()

	// Set the new commission
	senderAcct.DelegatorCommission, _ = value.Decimal().Float64()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	acctKeeper.Update(sender, senderAcct)

	return nil
}
