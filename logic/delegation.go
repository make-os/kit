package logic

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// execSetDelegatorCommission sets the delegator commission of an keystore
//
// ARGS:
// senderPubKey: The sender's public key
// value: The target commission (in percentage)
// fee: The fee paid by the sender
// chainHeight: The current chain height.
//
// EXPECT: Syntactic and consistency validation to have been performed by caller.
func (t *Transaction) execSetDelegatorCommission(
	senderPubKey util.Bytes32,
	value,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	acctKeeper := t.logic.AccountKeeper()

	// Get sender accounts
	sender := spk.Addr()
	senderAcct := acctKeeper.Get(sender)
	senderBal := senderAcct.Balance.Decimal()

	// Set the new commission
	senderAcct.DelegatorCommission, _ = value.Decimal().Float64()

	// Deduct the fee from the sender's keystore
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender keystore
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	return nil
}
