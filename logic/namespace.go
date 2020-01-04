package logic

import (
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/util"
)

// execPush executes a namespace acquisition request
//
// ARGS:
// spk: The sender's public key
// name: The hashed name
// value: The value to be paid for the namespace
// fee: The fee to be paid by the sender.
// transferToRepo: A target repository to transfer ownership of the namespace.
// transferToAccount: An address of an account to transfer ownership to.
// chainHeight: The chain height to limit query to.
//
//
// CONTRACT (caller must have met the following expectations):
// - Sender public key must be valid
func (t *Transaction) execAcquireNamespace(
	spk util.Bytes32,
	name string,
	value util.String,
	fee util.String,
	transferToRepo string,
	transferToAccount string,
	chainHeight uint64) error {

	spkObj := crypto.MustPubKeyFromBytes(spk.Bytes())

	// Get the current namespace object and re-populate it
	ns := t.logic.NamespaceKeeper().GetNamespace(name, chainHeight)
	ns.Owner = spkObj.Addr().String()
	ns.ExpiresAt = chainHeight + uint64(params.NamespaceTTL)
	ns.GraceEndAt = ns.ExpiresAt + uint64(params.NamespaceGraceDur)
	if transferToAccount != "" {
		ns.Owner = transferToAccount
	}
	if transferToRepo != "" {
		ns.Owner = transferToRepo
	}

	// Get the account of the sender
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.GetAccount(spkObj.Addr(), chainHeight)

	// Deduct the fee + value
	senderAcctBal := senderAcct.Balance.Decimal()
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderAcctBal.Sub(spendAmt).String())

	// Increment sender nonce, clean up and update
	senderAcct.Nonce = senderAcct.Nonce + 1
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(spkObj.Addr(), senderAcct)

	// Send the value to the treasury
	treasuryAcct := acctKeeper.GetAccount(params.TreasuryAddress, chainHeight)
	treasuryBal := treasuryAcct.Balance.Decimal()
	treasuryAcct.Balance = util.String(treasuryBal.Add(value.Decimal()).String())
	treasuryAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(params.TreasuryAddress, treasuryAcct)

	// Update the namespace
	t.logic.NamespaceKeeper().Update(name, ns)

	return nil
}
