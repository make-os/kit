package common

import (
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/shopspring/decimal"
)

const (
	ErrFailedToApplyProposal = "failed to apply proposal"
	ErrFailedToIndexProposal = "failed to index proposal against end height"
)

// DebitAccount debits an account of a specific amount.
// It increments the account's nonce and persist the updates.
func DebitAccount(keepers core.Keepers, targetAcct *ed25519.PubKey, amount decimal.Decimal, chainHeight uint64) {

	// Get the sender account and balance
	acctKeeper := keepers.AccountKeeper()
	senderAcct := acctKeeper.Get(targetAcct.Addr())
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the debitAmt from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(amount).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(targetAcct.Addr(), senderAcct)
}

// DebitAccountByAddress is like DebitAccount but accepts the address of the debit account.
func DebitAccountByAddress(keepers core.Keepers, targetAddr identifier.Address, amt decimal.Decimal, chainHeight uint64) {

	// Get the sender account and balance
	acctKeeper := keepers.AccountKeeper()
	senderAcct := acctKeeper.Get(targetAddr)
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the debitAmt from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(amt).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(targetAddr, senderAcct)
}

// DebitAccountObject is like DebitAccount, but it accepts the debit account object.
// It increments the account's nonce and persist the updates.
func DebitAccountObject(keepers core.Keepers,
	targetAddr identifier.Address,
	targetAcct *state.Account,
	amount decimal.Decimal,
	chainHeight uint64) {

	senderBal := targetAcct.Balance.Decimal()

	// Deduct the debitAmt from the sender's account
	targetAcct.Balance = util.String(senderBal.Sub(amount).String())

	// Increment nonce
	targetAcct.Nonce = targetAcct.Nonce + 1

	// Update the sender's account
	targetAcct.Clean(chainHeight)
	acctKeeper := keepers.AccountKeeper()
	acctKeeper.Update(targetAddr, targetAcct)
}
