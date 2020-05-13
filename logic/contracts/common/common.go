package common

import (
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// DebitAccount debits an account of a specific amount.
// It increments the account's nonce and persist the updates.
func DebitAccount(
	logic core.Logic,
	targetAcctPubKey *crypto.PubKey,
	amount decimal.Decimal,
	chainHeight uint64) {

	// Get the sender account and balance
	acctKeeper := logic.AccountKeeper()
	senderAcct := acctKeeper.Get(targetAcctPubKey.Addr())
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the debitAmt from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(amount).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(targetAcctPubKey.Addr(), senderAcct)
}

// DebitAccountObject is like DebitAccount, but it accepts an account object.
// It increments the account's nonce and persist the updates.
func DebitAccountObject(
	logic core.Logic,
	senderAcctAddr util.Address,
	senderAcct *state.Account,
	amount decimal.Decimal,
	chainHeight uint64) {

	senderBal := senderAcct.Balance.Decimal()

	// Deduct the debitAmt from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(amount).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper := logic.AccountKeeper()
	acctKeeper.Update(senderAcctAddr, senderAcct)
}
