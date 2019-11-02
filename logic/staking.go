package logic

import (
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// execUnbond sets an unbond height on a target stake
//
// ARG:
// ticketID: The target ticket ID
// senderPubKey: The public key of the tx sender.
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execUnbond(
	ticketID []byte,
	senderPubKey util.String,
	fee util.String,
	chainHeight uint64) error {

	// Get sender account
	acctKeeper := t.logic.AccountKeeper()
	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	senderAcct := acctKeeper.GetAccount(spk.Addr(), int64(chainHeight))
	senderBal := senderAcct.Balance.Decimal()

	// Get the ticket
	ticket := t.logic.GetTicketManager().GetByHash(string(ticketID))
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	// Set new unbond height
	newUnbondHeight := chainHeight + 1 + uint64(params.NumBlocksInStorerThawPeriod)
	senderAcct.Stakes.UpdateUnbondHeight(types.StakeTypeStorer,
		util.String(ticket.Value), 0, newUnbondHeight)

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	return nil
}

// execStorerStake sets aside some balance as storer stake.
//
// ARGS:
// senderPubKey: The public key of the tx sender.
// value: The value of the transaction.
// fee: The fee paid by the sender.
// nonce: The nonce of the transaction.
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execStorerStake(
	senderPubKey,
	value util.String,
	fee util.String,
	chainHeight uint64) error {
	return t.addStake(
		types.TxTypeStorerTicket,
		senderPubKey,
		value,
		fee,
		chainHeight,
	)
}

// execValidatorStake sets aside some balance as validator stake.
//
// ARGS:
// senderPubKey: The public key of the tx sender.
// value: The value of the transaction.
// fee: The fee paid by the sender.
// nonce: The nonce of the transaction.
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execValidatorStake(
	senderPubKey,
	value,
	fee util.String,
	chainHeight uint64) error {
	return t.addStake(
		types.TxTypeValidatorTicket,
		senderPubKey,
		value,
		fee,
		chainHeight,
	)
}

// addStake adds a stake entry to an account
//
// ARGS:
// txType: The transaction type
// senderPubKey: The public key of the tx sender.
// value: The value of the transaction.
// fee: The fee paid by the sender.
// nonce: The nonce of the transaction.
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) addStake(
	txType int,
	senderPubKey,
	value,
	fee util.String,
	chainHeight uint64) error {
	spk, _ := crypto.PubKeyFromBase58(senderPubKey.String())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender, int64(chainHeight))

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
	switch txType {
	case types.TxTypeValidatorTicket:
		senderAcct.Stakes.Add(types.StakeTypeValidator, value, unbondHeight)
	case types.TxTypeStorerTicket:
		senderAcct.Stakes.Add(types.StakeTypeStorer, value, 0)
	}

	// Update the sender account
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
