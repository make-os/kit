package logic

import (
	"fmt"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// execCoinTransfer transfers units of the native currency from a sender's account to another.
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
	recipientAddr util.Address,
	value util.String,
	fee util.String,
	chainHeight uint64) error {

	var recvAcct core.BalanceAccount
	recvAddr := recipientAddr
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

	// Check if the recipient address is a prefixed address (e.g r/repo or a/repo).
	// If so, we need to get the balance account object corresponding
	// to the actual resource name.
	if recvAddr.IsPrefixed() {
		resourceName := util.GetPrefixedAddressValue(recvAddr.String())
		recipientAddr = util.Address(resourceName)
		if util.IsPrefixedAddressRepo(recvAddr.String()) {
			recvAcct = repoKeeper.Get(resourceName)
		} else {
			recvAcct = acctKeeper.Get(util.Address(resourceName))
		}
	}

	// Check if the recipient address is a bech32 address.
	// If so, get the account object corresponding to the address.
	if recvAddr.IsBech32MakerAddress() {
		recvAcct = acctKeeper.Get(recipientAddr)
	}

	// Get the sender's account and balance
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	sender := spk.Addr()
	senderAcct := acctKeeper.Get(sender)
	senderBal := senderAcct.Balance.Decimal()

	// When the sender is also the recipient, use the sender account as recipient account
	if sender.Equal(recipientAddr) {
		recvAcct = senderAcct
	}

	// Calculate the spend amount and deduct the spend amount from
	// the sender's account, then increment sender's nonce
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderBal.Sub(spendAmt).String())
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Clean up and update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(sender, senderAcct)

	// Register the transaction value to the recipient balance
	recipientBal := recvAcct.GetBalance().Decimal()
	recvAcct.SetBalance(recipientBal.Add(value.Decimal()).String())

	// Clean up the recipient object
	recvAcct.Clean(chainHeight)

	// Save the new state of the object
	switch o := recvAcct.(type) {
	case *state.Account:
		acctKeeper.Update(recipientAddr, o)
	case *state.Repository:
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
// sender: The address of the sender or *crypto.PubKey of the sender
// value: The value of the transaction
// fee: The fee paid by the sender.
// chainHeight: The height of the block chain
func (t *Transaction) CanExecCoinTransfer(
	sender interface{},
	value,
	fee util.String,
	nonce,
	chainHeight uint64) error {

	senderAddr := ""
	switch o := sender.(type) {
	case *crypto.PubKey:
		senderAddr = o.Addr().String()
	case string:
		senderAddr = o
	case util.Address:
		senderAddr = o.String()
	default:
		panic("unsupported type")
	}

	// Get sender and recipient accounts
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.Get(util.Address(senderAddr))

	field := "value"
	if value == "0" && fee != "0" {
		field = "fee"
	}

	// Ensure the transaction nonce is the next expected nonce
	expectedNonce := senderAcct.Nonce + 1
	if expectedNonce != nonce {
		return util.FieldError(field, fmt.Sprintf("tx has invalid nonce (%d); expected (%d)",
			nonce, expectedNonce))
	}

	// Ensure sender has enough spendable balance to pay transfer value + fee
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderBal := senderAcct.GetSpendableBalance(chainHeight).Decimal()
	if !senderBal.GreaterThanOrEqual(spendAmt) {
		return util.FieldError(field, fmt.Sprintf("sender's spendable account "+
			"balance is insufficient"))
	}

	return nil
}
