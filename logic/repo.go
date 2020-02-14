package logic

import (
	"strings"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
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
	config map[string]interface{},
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(creatorPubKey.Bytes())

	// Create the repo object; Set the config to default if
	// the passed config is unset.
	newRepo := types.BareRepository()
	newRepo.Config = types.MakeDefaultRepoConfig()
	newRepo.Config.MergeMap(config)

	proposee := newRepo.Config.Governace.ProposalProposee

	// Add sender as owner only if proposee type is ProposeeOwner
	// Add sender as a veto owner if proposee type is ProposeeNetStakeholdersAndVetoOwner
	if proposee == types.ProposeeOwner || proposee == types.ProposeeNetStakeholdersAndVetoOwner {
		newRepo.AddOwner(spk.Addr().String(), &types.RepoOwner{
			Creator:  true,
			Veto:     proposee == types.ProposeeNetStakeholdersAndVetoOwner,
			JoinedAt: chainHeight + 1,
		})
	}

	t.logic.RepoKeeper().Update(name, newRepo)

	// Deduct fee from sender
	t.deductFee(spk, fee.Decimal(), chainHeight)

	return nil
}

// deductFee deducts the given fee from the account corresponding to the sender
// public key; It also increments the senders account nonce by 1.
func (t *Transaction) deductFee(spk *crypto.PubKey, fee decimal.Decimal, chainHeight uint64) {

	// Get the sender account and balance
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.GetAccount(spk.Addr())
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the fee from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(fee).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(spk.Addr(), senderAcct)
}

// applyProposalAddOwner adds the address described in the proposal as a repo owner.
func applyProposalAddOwner(
	proposal types.Proposal,
	repo *types.Repository,
	chainHeight uint64) error {

	// Get the action data
	targetAddrs := proposal.GetActionData()[types.ProposalActionDataAddresses].(string)
	veto := proposal.GetActionData()[types.ProposalActionDataVeto].(bool)

	// Add new repo owner iif the target address does not
	// already exist as an owner. If it exists, just update the fields.
	for _, address := range strings.Split(targetAddrs, ",") {

		existingOwner := repo.Owners.Get(address)
		if existingOwner != nil {
			existingOwner.Veto = veto
			continue
		}

		repo.AddOwner(address, &types.RepoOwner{
			Creator:  false,
			JoinedAt: chainHeight + 1,
			Veto:     veto,
		})
	}

	return nil
}

// applyProposalRepoUpdate updates a repo with data in the proposal.
func applyProposalRepoUpdate(
	proposal types.Proposal,
	repo *types.Repository,
	chainHeight uint64) error {
	cfgUpd := proposal.GetActionData()[types.ProposalActionDataConfig].(map[string]interface{})
	repo.Config.MergeMap(cfgUpd)
	return nil
}

// execRepoUpsertOwner adds or update a repository owner
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// addresses: The addresses of the owners.
// veto: whether to grant veto right
// proposalFee: The proposal anti-spam fee
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoUpsertOwner(
	senderPubKey util.Bytes32,
	repoName,
	proposalID,
	addresses string,
	veto bool,
	proposalFee util.String,
	fee util.String,
	chainHeight uint64) error {

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = types.ProposalActionAddOwner
	proposal.ActionData = map[string]interface{}{
		types.ProposalActionDataAddresses: addresses,
		types.ProposalActionDataVeto:      veto,
	}

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.deductFee(spk, totalFee, chainHeight)

	// Attempt to apply the proposal action
	applied, err := maybeApplyProposal(t.logic, proposal, repo, chainHeight)
	if err != nil {
		return errors.Wrap(err, "failed to apply proposal")
	} else if applied {
		goto update
	}

	// Index the proposal against its end height so it can be tracked
	// and finalized at that height.
	if err = repoKeeper.IndexProposalEnd(repoName, proposal.ID, proposal.EndAt); err != nil {
		return errors.Wrap(err, "failed to index proposal against end height")
	}

update:
	repoKeeper.Update(repoName, repo)
	return nil
}

// execRepoProposalUpdate creates a proposal to update a repository.
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// config: Updated repo configuration
// proposalFee: The proposal fee
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoProposalUpdate(
	senderPubKey util.Bytes32,
	repoName string,
	proposalID string,
	config map[string]interface{},
	proposalFee util.String,
	fee util.String,
	chainHeight uint64) error {

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = types.ProposalActionRepoUpdate
	proposal.ActionData[types.ProposalActionDataConfig] = config

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.deductFee(spk, totalFee, chainHeight)

	// Attempt to apply the proposal action
	applied, err := maybeApplyProposal(t.logic, proposal, repo, chainHeight)
	if err != nil {
		return errors.Wrap(err, "failed to apply proposal")
	} else if applied {
		goto update
	}

	// Index the proposal against its end height so it
	// can be tracked and finalized at that height.
	if err = repoKeeper.IndexProposalEnd(repoName, proposal.ID, proposal.EndAt); err != nil {
		return errors.Wrap(err, "failed to index proposal against end height")
	}

update:
	repoKeeper.Update(repoName, repo)
	return nil
}

// execRepoProposalSendFee adds proposal fee
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// proposalID: The identity of the proposal
// proposalFee: The proposal anti-spam fee
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoProposalSendFee(
	senderPubKey util.Bytes32,
	repoName string,
	proposalID string,
	proposalFee util.String,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())

	// Get the repo and proposal
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName)
	prop := repo.Proposals.Get(proposalID)

	// Add proposal fee if set.
	// If the sender already deposited, update their deposit.
	if proposalFee != "0" {
		addr := spk.Addr().String()
		if !prop.Fees.Has(addr) {
			prop.Fees.Add(addr, proposalFee.String())
		} else {
			existingFee := prop.Fees.Get(addr)
			updFee := existingFee.Decimal().Add(proposalFee.Decimal())
			prop.Fees.Add(addr, updFee.String())
		}
	}

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.deductFee(spk, totalFee, chainHeight)

	repoKeeper.Update(repoName, repo)

	return nil
}

func makeProposal(
	spk *crypto.PubKey,
	repo *types.Repository,
	id string,
	proposalFee util.String,
	chainHeight uint64) *types.RepoProposal {

	proposal := &types.RepoProposal{
		ID:         id,
		Config:     repo.Config.Clone().Governace,
		Creator:    spk.Addr().String(),
		Height:     chainHeight,
		EndAt:      repo.Config.Governace.ProposalDur + chainHeight + 1,
		Fees:       map[string]string{},
		ActionData: map[string]interface{}{},
	}

	// Add proposal fee if set
	if proposalFee != "0" {
		proposal.Fees.Add(spk.Addr().String(), proposalFee.String())
	}

	// Set the max. join height for voters.
	if repo.Config.Governace.ProposalProposeeLimitToCurHeight {
		proposal.ProposeeMaxJoinHeight = chainHeight + 1
	}

	// Set the fee deposit end height and also update the proposal end height to
	// be after the fee deposit height
	if repo.Config.Governace.ProposalFeeDepDur > 0 {
		proposal.FeeDepositEndAt = 1 + chainHeight + repo.Config.Governace.ProposalFeeDepDur
		proposal.EndAt = proposal.FeeDepositEndAt + repo.Config.Governace.ProposalDur
	}

	// Add the proposal to the repo
	repo.Proposals.Add(proposal.ID, proposal)

	return proposal
}

// execRepoProposalMergeRequest creates a proposal to update a repository.
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// baseBranch: The base branch
// baseBranchHash: The base branch hash
// targetBranch: The target branch
// targetBranchHash: The target branch hash
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoProposalMergeRequest(
	senderPubKey util.Bytes32,
	repoName,
	proposalID,
	baseBranch,
	baseBranchHash,
	targetBranch,
	targetBranchHash string,
	proposalFee,
	fee util.String,
	chainHeight uint64) error {

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = types.ProposalActionMergeRequest
	proposal.ActionData[types.ProposalActionDataMergeRequest] = map[string]string{
		"base":       baseBranch,
		"baseHash":   baseBranchHash,
		"target":     targetBranch,
		"targetHash": targetBranchHash,
	}

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.deductFee(spk, totalFee, chainHeight)

	// Attempt to apply the proposal action
	applied, err := maybeApplyProposal(t.logic, proposal, repo, chainHeight)
	if err != nil {
		return errors.Wrap(err, "failed to apply proposal")
	} else if applied {
		goto update
	}

	// Index the proposal against its end height so it can be tracked and
	// finalized at that height.
	if err = repoKeeper.IndexProposalEnd(repoName, proposal.ID, proposal.EndAt); err != nil {
		return errors.Wrap(err, "failed to index proposal against end height")
	}

update:
	repoKeeper.Update(repoName, repo)
	return nil
}

// execRepoProposalVote processes votes on a repository proposal
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// proposalID: The identity of the proposal
// vote: Indicates the vote choice
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoProposalVote(
	senderPubKey util.Bytes32,
	repoName string,
	proposalID string,
	vote int,
	fee util.String,
	chainHeight uint64) error {

	var err error
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.GetRepo(repoName)
	prop := repo.Proposals.Get(proposalID)

	increments := float64(0)

	// When proposees are the owners, and tally method is ProposalTallyMethodIdentity
	// each proposee will have 1 voting power.
	if prop.Config.ProposalProposee == types.ProposeeOwner &&
		prop.Config.ProposalTallyMethod == types.ProposalTallyMethodIdentity {
		increments = 1
	}

	// When proposees are the owners, and tally method is ProposalTallyMethodCoinWeighted
	// each proposee will use the value of the voter's account spendable balance
	// as their voting power.
	if prop.Config.ProposalProposee == types.ProposeeOwner &&
		prop.Config.ProposalTallyMethod == types.ProposalTallyMethodCoinWeighted {
		senderAcct := t.logic.AccountKeeper().GetAccount(spk.Addr())
		increments = senderAcct.GetSpendableBalance(chainHeight).Float()
	}

	// For network staked-weighted votes, use the total value of coins directly
	// staked by the voter as their vote power
	if prop.Config.ProposalTallyMethod == types.ProposalTallyMethodNetStakeOfProposer {
		increments, err = t.logic.GetTicketManager().
			ValueOfNonDelegatedTickets(senderPubKey, prop.ProposeeMaxJoinHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get value of non-delegated tickets of sender")
		}
	}

	// For network staked-weighted votes, use the total value of coins delegated
	// to the voter as their vote power
	if prop.Config.ProposalTallyMethod == types.ProposalTallyMethodNetStakeOfDelegators {
		increments, err = t.logic.GetTicketManager().
			ValueOfDelegatedTickets(senderPubKey, prop.ProposeeMaxJoinHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get value of delegated tickets of sender")
		}
	}

	// For network staked-weighted votes, use the total value of coins delegated
	// to the voter as their vote power
	if prop.Config.ProposalTallyMethod == types.ProposalTallyMethodNetStake {

		tickets, err := t.logic.GetTicketManager().
			GetNonDecayedTickets(senderPubKey, prop.ProposeeMaxJoinHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get non-decayed tickets assigned to sender")
		}

		// Calculate the sum of value of all tickets.
		// For delegated tickets, check whether the delegator already voted. If
		// yes, do not count their ticket.
		sumValue := decimal.Zero
		for _, ticket := range tickets {

			proposerPK := ticket.ProposerPubKey

			// Count the ticket if it is not delegated or the delegator is also the voter
			if ticket.Delegator == "" || (ticket.Delegator == spk.Addr().String() &&
				proposerPK.Equal(senderPubKey)) {
				sumValue = sumValue.Add(ticket.Value.Decimal())
				continue
			}

			// For tickets not delegated by the voter, determine whether the
			// delegator has used their ticket to vote on this same proposal.
			// If yes, we will not count it.
			if ticket.Delegator != spk.Addr().String() {
				_, voted, err := repoKeeper.GetProposalVote(repoName, proposalID, ticket.Delegator)
				if err != nil {
					return errors.Wrap(err, "failed to check ticket's delegator vote status")
				}
				if !voted {
					sumValue = sumValue.Add(ticket.Value.Decimal())
					continue
				}
			}

			// For tickets delegated by the voter to a different user,
			// determine if ticket proposer has voted in this same proposal.
			// If yes, deduct the vote and apply to the delegator's choice vote option
			if ticket.Delegator == spk.Addr().String() {
				proposerAddr := crypto.MustPubKeyFromBytes(proposerPK.Bytes()).Addr().String()
				vote, voted, err := repoKeeper.GetProposalVote(repoName, proposalID, proposerAddr)
				if err != nil {
					return errors.Wrap(err, "failed to check ticket's proposer vote status")
				}
				if !voted {
					sumValue = sumValue.Add(ticket.Value.Decimal())
					continue
				}

				switch vote {
				case types.ProposalVoteYes:
					newYes := decimal.NewFromFloat(prop.Yes)
					newYes = newYes.Sub(ticket.Value.Decimal())
					prop.Yes, _ = newYes.Float64()

				case types.ProposalVoteNo:
					newNo := decimal.NewFromFloat(prop.No)
					newNo = newNo.Sub(ticket.Value.Decimal())
					prop.Yes, _ = newNo.Float64()

				case types.ProposalVoteNoWithVeto:
					newNoWithVeto := decimal.NewFromFloat(prop.NoWithVeto)
					newNoWithVeto = newNoWithVeto.Sub(ticket.Value.Decimal())
					prop.NoWithVeto, _ = newNoWithVeto.Float64()

				case types.ProposalVoteAbstain:
					newAbstain := decimal.NewFromFloat(prop.Abstain)
					newAbstain = newAbstain.Sub(ticket.Value.Decimal())
					prop.Abstain, _ = newAbstain.Float64()
				}

				sumValue = sumValue.Add(ticket.Value.Decimal())
			}
		}

		increments, _ = sumValue.Float64()
	}

	if vote == types.ProposalVoteYes {
		prop.Yes += increments
	} else if vote == types.ProposalVoteNo {
		prop.No += increments
	} else if vote == types.ProposalVoteAbstain {
		prop.Abstain += increments
	} else if vote == types.ProposalVoteNoWithVeto {
		prop.NoWithVeto += increments

		// Also, if the proposee type for the proposal is stakeholders and veto
		// owners and voter is an owner, increment NoWithVetoByOwners by 1
		voterAsOwner := repo.Owners.Get(spk.Addr().String())
		isStakeholderAndVetoOwnerProposee := prop.Config.ProposalProposee ==
			types.ProposeeNetStakeholdersAndVetoOwner
		if isStakeholderAndVetoOwnerProposee && voterAsOwner != nil && voterAsOwner.Veto {
			prop.NoWithVetoByOwners = 1
		}
	}

	// Update the repo
	repoKeeper.Update(repoName, repo)

	// Deduct fee from sender
	t.deductFee(spk, fee.Decimal(), chainHeight)

	return nil
}
