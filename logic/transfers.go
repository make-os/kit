package logic

import (
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

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
	senderPubKey util.Bytes32,
	recipientAddr,
	value util.String,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender account and balance
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender, chainHeight)
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the spend amount from the sender's account and increment nonce
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up unbonded stakes and update sender account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	// Get recipient account only if recipient and sender are different,
	// otherwise use the sender account as recipient account
	var recipientAcct = senderAcct
	if !sender.Equal(recipientAddr) {
		recipientAcct = acctKeeper.GetAccount(recipientAddr, chainHeight)
	}

	// Add the transaction value to the recipient balance
	recipientBal := recipientAcct.Balance.Decimal()
	recipientAcct.Balance = util.String(recipientBal.Add(value.Decimal()).String())

	// Clean up unbonded stakes and update recipient account
	recipientAcct.Clean(chainHeight)
	acctKeeper.Update(recipientAddr, recipientAcct)

	return nil
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
	senderAcct := acctKeeper.GetAccount(sender, chainHeight)

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
