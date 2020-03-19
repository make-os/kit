package logic

import (
	"fmt"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// execUnbond sets an unbond height on a target stake
//
// ARG:
// senderPubKey: The public key of the tx sender.
// ticketID: The target ticket ID
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execUnbond(
	senderPubKey util.Bytes32,
	ticketID util.Bytes32,
	fee util.String,
	chainHeight uint64) error {

	// Get sender account
	acctKeeper := t.logic.AccountKeeper()
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	senderAcct := acctKeeper.Get(spk.Addr(), chainHeight)
	senderBal := senderAcct.Balance.Decimal()

	// Get the ticket
	ticket := t.logic.GetTicketManager().GetByHash(ticketID)
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	// Set new unbond height
	newUnbondHeight := chainHeight + 1 + uint64(params.NumBlocksInHostThawPeriod)
	senderAcct.Stakes.UpdateUnbondHeight(state.StakeTypeHost,
		util.String(ticket.Value), 0, newUnbondHeight)

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	return nil
}

// execHostStake sets aside some balance as host stake.
//
// ARGS:
// senderPubKey: The public key of the tx sender.
// value: The value of the transaction.
// fee: The fee paid by the sender.
// nonce: The nonce of the transaction.
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execHostStake(
	senderPubKey util.Bytes32,
	value util.String,
	fee util.String,
	chainHeight uint64) error {
	return t.addStake(
		core.TxTypeHostTicket,
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
	senderPubKey util.Bytes32,
	value,
	fee util.String,
	chainHeight uint64) error {
	return t.addStake(
		core.TxTypeValidatorTicket,
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
	senderPubKey util.Bytes32,
	value,
	fee util.String,
	chainHeight uint64) error {
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.Get(sender)

	// Deduct the transaction fee and increment nonce
	senderBal := senderAcct.Balance.Decimal()
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	unbondHeight := uint64(0)

	// Register a stake entry
	switch txType {
	case core.TxTypeValidatorTicket:
		// Determine unbond height. The unbond height is height of the next block
		// (or proposed block) plus minimum ticket maturation duration, max ticket
		// active duration + thawing period.
		unbondHeight = chainHeight + 1 + uint64(params.MinTicketMatDur) +
			uint64(params.MaxTicketActiveDur) +
			uint64(params.NumBlocksInThawPeriod)
		senderAcct.Stakes.Add(state.StakeTypeValidator, value, unbondHeight)

	case core.TxTypeHostTicket:
		senderAcct.Stakes.Add(state.StakeTypeHost, value, unbondHeight)
	}

	// Update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
