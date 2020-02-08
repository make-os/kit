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
// recipientAddr: The recipient address (can be base58, namespaced URI or prefixed address)
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

	var recvAcct types.BalanceAccount
	recvAddr := recipientAddr.Address()
	acctKeeper := t.logic.AccountKeeper()
	repoKeeper := t.logic.RepoKeeper()

	// Check if the recipient address is a namespace URI. If so,
	// we need to resolve it to the target address which is expected
	// to be a prefixed address.
	if recvAddr.IsNamespaceURI() {
		target, err := t.logic.NamespaceKeeper().GetTarget(recvAddr.String())
		if err != nil {
			return err
		}
		recvAddr = util.Address(target)
	}

	// Check if recipient address is a prefixed address (e.g r/repo or a/repo).
	// If so, we need to get the balance account object corresponding
	// to the actual resource name.
	if recvAddr.IsPrefixed() {
		resourceName := util.GetPrefixedAddressValue(recvAddr.String())
		recipientAddr = util.String(resourceName)
		if util.IsPrefixedAddressRepo(recvAddr.String()) {
			recvAcct = repoKeeper.GetRepo(resourceName)
		} else {
			recvAcct = acctKeeper.GetAccount(util.String(resourceName))
		}
	}

	// Check if the recipient address is a base58 encoded address.
	// If so, get the account object corresponding to the address.
	if recvAddr.IsBase58Address() {
		recvAcct = acctKeeper.GetAccount(recipientAddr)
	}

	// Get the sender account and balance
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	sender := spk.Addr()
	senderAcct := acctKeeper.GetAccount(sender)
	senderBal := senderAcct.Balance.Decimal()

	// When the sender is also the recipient, use sender
	// account as recipient account
	if sender.Equal(recipientAddr) {
		recvAcct = senderAcct
	}

	// Calculate the spend amount and deduct the spend amount from
	// the sender's account, then increment sender's nonce
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up and update the sender account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	// Add the transaction value to the recipient balance
	recipientBal := recvAcct.GetBalance().Decimal()
	recvAcct.SetBalance(recipientBal.Add(value.Decimal()).String())

	// Clean up the recipient object
	recvAcct.Clean(chainHeight)

	// Save the new state of the object
	switch o := recvAcct.(type) {
	case *types.Account:
		acctKeeper.Update(recipientAddr, o)
	case *types.Repository:
		repoKeeper.Update(recipientAddr.String(), o)
	}

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
	senderAcct := acctKeeper.GetAccount(sender)

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
