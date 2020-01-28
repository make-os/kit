package logic

import (
	"fmt"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
)

// execRepoCreate processes a TxTypeRepoCreate transaction, which creates a git
// repository.
//
// ARGS:
// creatorPubKey: The public key of the creator
// name: The name of the repository
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Creator's public key must be valid
func (t *Transaction) execRepoCreate(
	creatorPubKey util.Bytes32,
	name string,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(creatorPubKey.Bytes())

	// Create the repo object
	newRepo := types.BareRepository()
	newRepo.Config = types.DefaultRepoConfig()
	newRepo.AddOwner(spk.Addr().String(), &types.RepoOwner{
		Creator:  true,
		JoinedAt: chainHeight + 1,
	})
	t.logic.RepoKeeper().Update(name, newRepo)

	// Deduct fee from sender
	t.deductFee(spk, fee, chainHeight)

	return nil
}

// applyAddOwnerProposal adds the address described in the proposal as a repo owner.
func applyAddOwnerProposal(
	proposal types.Proposal,
	repo *types.Repository,
	chainHeight uint64) error {

	targetAddr := proposal.GetActionData()["address"].(string)

	// Add new repo owner if not target address isn't already an owner
	if !repo.Owners.Has(targetAddr) {
		repo.AddOwner(targetAddr, &types.RepoOwner{
			Creator:  false,
			JoinedAt: chainHeight + 1,
		})
		return nil
	}

	// ..Update specific fields if set

	return nil
}

// deductFee deducts the given fee from the account corresponding to the sender
// public key; It also increments the senders account nonce by 1.
func (t *Transaction) deductFee(spk *crypto.PubKey, fee util.String, chainHeight uint64) {

	// Get the sender account and balance
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.GetAccount(spk.Addr(), chainHeight)
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee.Decimal()).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)
}

// execRepoUpsertOwner processes a TxTypeRepoCreate transaction, which creates a git
// repository.
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// ownerAddress: The address of the owner.
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoUpsertOwner(
	senderPubKey util.Bytes32,
	txID string,
	repoName string,
	ownerAddress string,
	fee util.String,
	chainHeight uint64) error {

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := &types.RepoProposal{
		Action:      types.ProposalActionAddOwner,
		Creator:     spk.Addr().String(),
		Proposee:    repo.Config.Governace.ProposalProposee,
		ProposeeAge: repo.Config.Governace.ProposalProposeeExistBeforeHeight,
		EndAt:       repo.Config.Governace.ProposalDur + chainHeight + 1,
		TallyMethod: repo.Config.Governace.ProposalTallyMethod,
		Quorum:      repo.Config.Governace.ProposalQuorum,
		Threshold:   repo.Config.Governace.ProposalThreshold,
		VetoQuorum:  repo.Config.Governace.ProposalVetoQuorum,
		ActionData: map[string]interface{}{
			"address": ownerAddress,
		},
	}

	// Add the proposal to the repo (strip 0x from tx ID)
	proposalID := fmt.Sprintf("%d", len(repo.Proposals)+1)
	repo.Proposals.Add(proposalID, proposal)

	// Attempt to apply the proposal action
	applied, err := maybeApplyProposal(t.logic, proposal, repo, chainHeight)
	if err != nil {
		return errors.Wrap(err, "failed to apply proposal")
	} else if applied {
		proposal.SetFinalized(true)
		proposal.SetSelfAccepted(true)
	}

	// Update the repo
	repoKeeper.Update(repoName, repo)

	// Deduct fee from sender
	t.deductFee(spk, fee, chainHeight)

	return nil
}
