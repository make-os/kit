package logic

import (
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// execPush executes a push transaction
//
// ARGS:
// repoName: The name of the target repo
// references: The pushed references
// endorsements: The endorsements by hosts
// fee: The fee paid by the sender
// pusherKeyID: The id of the pusher
// chainHeight: The chain height to limit query to
//
// CONTRACT (caller must have met the following expectations):
// - Repo must exist
// - Pusher GPG key must exist
func (t *Transaction) execPush(
	repoName string,
	references core.PushedReferences,
	fee util.String,
	pusherKeyID []byte,
	chainHeight uint64) error {

	// Get repository
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.Get(repoName)

	// Get the GPG public key of the pusher
	gpgID := util.MustCreateGPGID(pusherKeyID)
	gpgPK := t.logic.GPGPubKeyKeeper().Get(gpgID, chainHeight)

	// Register the references to the repo and update their nonce
	for _, ref := range references {
		curRef := repo.References.Get(ref.Name)
		curRef.Nonce = curRef.Nonce + 1
		repo.References[ref.Name] = curRef
	}

	// Get the keystore of the pusher
	acctKeeper := t.logic.AccountKeeper()
	pusherAcct := acctKeeper.Get(gpgPK.Address)

	// Deduct the fee
	pusherAcctBal := pusherAcct.Balance.Decimal()
	spendAmt := fee.Decimal()
	pusherAcct.Balance = util.String(pusherAcctBal.Sub(spendAmt).String())
	pusherAcct.Nonce = pusherAcct.Nonce + 1

	// Clean up unbonded stakes and update sender keystore
	pusherAcct.Clean(chainHeight)
	acctKeeper.Update(gpgPK.Address, pusherAcct)

	// Update the repo
	repoKeeper.Update(repoName, repo)

	return nil
}
