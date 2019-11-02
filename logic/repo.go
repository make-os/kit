package logic

import (
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// execRepoCreate processes a TxTypeRepoCreate transaction, which creates a git
// repository.
//
// ARGS:
// creatorPubKey: The public key of the creator
// name: The name of the repository
//
// CONTRACT: Creator's public key must be valid
func (t *Transaction) execRepoCreate(
	creatorPubKey util.String,
	name string,
	fee util.String,
	chainHeight uint64) error {

	// Create the repo object
	newRepo := types.BareRepository()
	newRepo.CreatorPubKey = creatorPubKey
	t.logic.RepoKeeper().Update(name, newRepo)

	// Get the sender account and balance
	acctKeeper := t.logic.AccountKeeper()
	spk, _ := crypto.PubKeyFromBase58(creatorPubKey.String())
	senderAcct := acctKeeper.GetAccount(spk.Addr(), int64(chainHeight))
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.CleanUnbonded(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)

	return nil
}
