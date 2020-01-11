package logic

import (
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// execPush executes a push transaction
//
// ARGS:
// repoName: The name of the target repo
// references: The pushed references
// endorsements: The endorsements by storers
// fee: The fee paid by the sender
// pusherKeyID: The id of the pusher
// chainHeight: The chain height to limit query to
//
// CONTRACT (caller must have met the following expectations):
// - Repo must exist
// - Pusher GPG key must exist
func (t *Transaction) execPush(
	repoName string,
	references types.PushedReferences,
	fee util.String,
	pusherKeyID []byte,
	chainHeight uint64) error {

	// Get repository
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName, chainHeight)

	// Get the GPG public key of the pusher
	gpgPK := t.logic.GPGPubKeyKeeper().GetGPGPubKey(util.ToHex(pusherKeyID), chainHeight)

	// Add the references to the repo and update their nonce
	for _, ref := range references {
		curRef := repo.References.Get(ref.Name)
		curRef.Nonce = curRef.Nonce + 1
		repo.References[ref.Name] = curRef
	}

	// Get the account of the pusher
	acctKeeper := t.logic.AccountKeeper()
	pusherAcct := acctKeeper.GetAccount(gpgPK.Address, chainHeight)

	// Deduct the fee
	pusherAcctBal := pusherAcct.Balance.Decimal()
	spendAmt := fee.Decimal()
	pusherAcct.Balance = util.String(pusherAcctBal.Sub(spendAmt).String())
	pusherAcct.Nonce = pusherAcct.Nonce + 1

	// Clean up unbonded stakes and update sender account
	pusherAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(gpgPK.Address, pusherAcct)

	// Update the repo
	repoKeeper.Update(repoName, repo)

	return nil
}
