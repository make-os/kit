package logic

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"
)

// execPush executes a namespace acquisition request
//
// ARGS:
// creatorPubKey: The sender's public key
// name: The hashed name
// value: The value to be paid for the namespace
// fee: The fee to be paid by the sender.
// transferToRepo: A target repository to transfer ownership of the namespace.
// transferToAccount: An address of an account to transfer ownership to.
// domain: The namespace domains
// chainHeight: The chain height to limit query to.
//
//
// CONTRACT (caller must have met the following expectations):
// - Sender public key must be valid
func (t *Transaction) execAcquireNamespace(
	creatorPubKey util.Bytes32,
	name string,
	value util.String,
	fee util.String,
	transferToRepo string,
	transferToAccount string,
	domains map[string]string,
	chainHeight uint64) error {

	spk := crypto.MustPubKeyFromBytes(creatorPubKey.Bytes())

	// Get the current namespace object and re-populate it
	ns := t.logic.NamespaceKeeper().Get(name)
	ns.Owner = spk.Addr().String()
	ns.ExpiresAt = chainHeight + uint64(params.NamespaceTTL)
	ns.GraceEndAt = ns.ExpiresAt + uint64(params.NamespaceGraceDur)
	ns.Domains = domains
	if transferToAccount != "" {
		ns.Owner = transferToAccount
	}
	if transferToRepo != "" {
		ns.Owner = transferToRepo
	}

	// Get the account of the sender
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.GetAccount(spk.Addr())

	// Deduct the fee + value
	senderAcctBal := senderAcct.Balance.Decimal()
	spendAmt := value.Decimal().Add(fee.Decimal())
	senderAcct.Balance = util.String(senderAcctBal.Sub(spendAmt).String())

	// Increment sender nonce, clean up and update
	senderAcct.Nonce = senderAcct.Nonce + 1
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	// Send the value to the treasury
	treasuryAcct := acctKeeper.GetAccount(util.String(params.TreasuryAddress), chainHeight)
	treasuryBal := treasuryAcct.Balance.Decimal()
	treasuryAcct.Balance = util.String(treasuryBal.Add(value.Decimal()).String())
	treasuryAcct.Clean(chainHeight)
	acctKeeper.Update(util.String(params.TreasuryAddress), treasuryAcct)

	// Update the namespace
	t.logic.NamespaceKeeper().Update(name, ns)

	return nil
}

// execUpdateNamespaceDomains executes a namespace domain update
//
// ARGS:
// creatorPubKey: The sender's public key
// name: The hashed name
// value: The value to be paid for the namespace
// fee: The fee to be paid by the sender.
// transferToRepo: A target repository to transfer ownership of the namespace.
// transferToAccount: An address of an account to transfer ownership to.
// domain: The namespace domains
// chainHeight: The chain height to limit query to.
//
//
// CONTRACT (caller must have met the following expectations):
// - Sender public key must be valid
func (t *Transaction) execUpdateNamespaceDomains(
	creatorPubKey util.Bytes32,
	name string,
	fee util.String,
	domain map[string]string,
	chainHeight uint64) error {

	spk := crypto.MustPubKeyFromBytes(creatorPubKey.Bytes())

	// Get the current namespace object and update.
	// Remove existing domain if it is referenced in the update and has not target.
	ns := t.logic.NamespaceKeeper().Get(name)
	for domain, target := range domain {
		if _, ok := ns.Domains[domain]; !ok {
			ns.Domains[domain] = target
			continue
		}
		if target != "" {
			ns.Domains[domain] = target
			continue
		}
		delete(ns.Domains, domain)
	}

	// Update the namespace
	t.logic.NamespaceKeeper().Update(name, ns)

	// Get the account of the sender
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.GetAccount(spk.Addr())

	// Deduct the fee + value
	senderAcctBal := senderAcct.Balance.Decimal()
	spendAmt := fee.Decimal()
	senderAcct.Balance = util.String(senderAcctBal.Sub(spendAmt).String())

	// Increment sender nonce, clean up and update
	senderAcct.Nonce = senderAcct.Nonce + 1
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	return nil
}
