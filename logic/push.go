package logic

import (
	"gitlab.com/makeos/mosdef/crypto"
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
// - Pusher's push key key must exist
func (t *Transaction) execPush(
	repoName string,
	references core.PushedReferences,
	fee util.String,
	pushKeyID []byte,
	chainHeight uint64) error {

	// Get repository
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.Get(repoName)

	// Register the references to the repo and update their nonce
	for _, ref := range references {
		curRef := repo.References.Get(ref.Name)

		// When the reference should be deleted, remove from repo reference
		if ref.IsDeletable() && !curRef.IsNil() {
			delete(repo.References, ref.Name)
			continue
		}

		// Set pusher as creator if reference is new
		if curRef.IsNil() {
			curRef.Creator = pushKeyID
		}

		curRef.Nonce = curRef.Nonce + 1
		curRef.Hash = util.MustFromHex(ref.NewHash)
		repo.References[ref.Name] = curRef
	}

	// Get the push key of the pusher
	pushKey := t.logic.PushKeyKeeper().Get(crypto.BytesToPushKeyID(pushKeyID), chainHeight)

	// Get the account of the pusher
	acctKeeper := t.logic.AccountKeeper()
	pusherAcct := acctKeeper.Get(pushKey.Address)

	// Deduct the fee
	pusherAcctBal := pusherAcct.Balance.Decimal()
	spendAmt := fee.Decimal()
	pusherAcct.Balance = util.String(pusherAcctBal.Sub(spendAmt).String())
	pusherAcct.Nonce = pusherAcct.Nonce + 1

	// Clean up unbonded stakes and update sender account
	pusherAcct.Clean(chainHeight)
	acctKeeper.Update(pushKey.Address, pusherAcct)

	// Update the repo
	repoKeeper.Update(repoName, repo)

	return nil
}
