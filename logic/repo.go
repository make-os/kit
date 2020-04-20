package logic

import (
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// AddDefaultPolicies adds default repo-level policies
func AddDefaultPolicies(config *state.RepoConfig) {
	config.Policies = append(
		config.Policies,
		&state.Policy{Subject: "all", Object: "refs/heads", Action: "update"},
		&state.Policy{Subject: "all", Object: "refs/heads", Action: "merge-update"},
		&state.Policy{Subject: "all", Object: "refs/heads", Action: "issue-update"},
		&state.Policy{Subject: "all", Object: "refs/tags", Action: "update"},
		&state.Policy{Subject: "all", Object: "refs/notes", Action: "update"},
		&state.Policy{Subject: "all", Object: "refs/heads", Action: "delete"},
		&state.Policy{Subject: "all", Object: "refs/tags", Action: "delete"},
		&state.Policy{Subject: "all", Object: "refs/notes", Action: "delete"},
		&state.Policy{Subject: "all", Object: "refs/heads/master", Action: "deny-delete"},
	)
}

// execRepoCreate processes a TxTypeRepoCreate transaction, which creates a git
// repository.
//
// ARGS:
// creatorPubKey: The public key of the creator
// name: The name of the repository
// fee: The fee to be paid by the sender.
// config: The repo config
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
	newRepo := state.BareRepository()
	newRepo.Config = state.MakeDefaultRepoConfig()
	newRepo.Config.MergeMap(config)

	// Apply default policies when none is set
	if len(newRepo.Config.Policies) == 0 {
		AddDefaultPolicies(newRepo.Config)
	}

	voterType := newRepo.Config.Governance.Voter

	// Register sender as owner only if proposer type is ProposerOwner
	// Register sender as a veto owner if proposer type is ProposerNetStakeholdersAndVetoOwner
	if voterType == state.VoteByOwner || voterType == state.VoteByNetStakersAndVetoOwner {
		newRepo.AddOwner(spk.Addr().String(), &state.RepoOwner{
			Creator:  true,
			Veto:     voterType == state.VoteByNetStakersAndVetoOwner,
			JoinedAt: chainHeight + 1,
		})
	}

	t.logic.RepoKeeper().Update(name, newRepo)

	// Deduct fee from sender
	t.debitAccount(spk, fee.Decimal(), chainHeight)

	return nil
}

// debitAccount deducts the given amount from an account,
// increments its nonce and saves the updates.
// ARGS:
// spk: The public key of the target account
// debitAmt: The amount to be debited
// chainHeight: The current chain height
func (t *Transaction) debitAccount(acctPubKey *crypto.PubKey, debitAmt decimal.Decimal, chainHeight uint64) {

	// Get the sender account and balance
	acctKeeper := t.logic.AccountKeeper()
	senderAcct := acctKeeper.Get(acctPubKey.Addr())
	senderBal := senderAcct.Balance.Decimal()

	// Deduct the debitAmt from the sender's account
	senderAcct.Balance = util.String(senderBal.Sub(debitAmt).String())

	// Increment nonce
	senderAcct.Nonce = senderAcct.Nonce + 1

	// Update the sender's account
	senderAcct.Clean(chainHeight)
	acctKeeper.Update(acctPubKey.Addr(), senderAcct)
}

// applyProposalUpsertOwner adds the address described in the proposal as a repo owner.
//noinspection ALL
func applyProposalUpsertOwner(
	proposal state.Proposal,
	repo *state.Repository,
	chainHeight uint64) error {

	// Get the action data
	ad := proposal.GetActionData()
	var targetAddrs []string
	util.ToObject(ad[constants.ActionDataKeyAddrs], &targetAddrs)
	var veto bool
	util.ToObject(ad[constants.ActionDataKeyVeto], &veto)

	// Register new repo owner iif the target address does not
	// already exist as an owner. If it exists, just update select fields.
	for _, address := range targetAddrs {
		existingOwner := repo.Owners.Get(address)
		if existingOwner != nil {
			existingOwner.Veto = veto
			continue
		}

		repo.AddOwner(address, &state.RepoOwner{
			Creator:  false,
			JoinedAt: chainHeight + 1,
			Veto:     veto,
		})
	}

	return nil
}

// execRepoProposalUpsertOwner adds or update a repository owner
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
func (t *Transaction) execRepoProposalUpsertOwner(
	senderPubKey util.Bytes32,
	repoName,
	proposalID string,
	addresses []string,
	veto bool,
	proposalFee util.String,
	fee util.String,
	chainHeight uint64) error {

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.Get(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = core.TxTypeRepoProposalUpsertOwner
	proposal.ActionData = map[string][]byte{
		constants.ActionDataKeyAddrs: util.ToBytes(addresses),
		constants.ActionDataKeyVeto:  util.ToBytes(veto),
	}

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.debitAccount(spk, totalFee, chainHeight)

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

// applyProposalRepoUpdate updates a repo with data in the proposal.
func applyProposalRepoUpdate(
	proposal state.Proposal,
	repo *state.Repository,
	chainHeight uint64) error {

	var cfgUpd map[string]interface{}
	util.ToObject(proposal.GetActionData()[constants.ActionDataKeyCFG], &cfgUpd)

	// Merge update to existing config
	repo.Config.MergeMap(cfgUpd)

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
	repo := repoKeeper.Get(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = core.TxTypeRepoProposalUpdate
	proposal.ActionData[constants.ActionDataKeyCFG] = util.ToBytes(config)

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.debitAccount(spk, totalFee, chainHeight)

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

// execRepoProposalFeeDeposit adds proposal fee
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
func (t *Transaction) execRepoProposalFeeDeposit(
	senderPubKey util.Bytes32,
	repoName string,
	proposalID string,
	proposalFee util.String,
	fee util.String,
	chainHeight uint64) error {

	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())

	// Get the repo and proposal
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.Get(repoName)
	prop := repo.Proposals.Get(proposalID)

	// Register proposal fee if set.
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
	t.debitAccount(spk, totalFee, chainHeight)

	repoKeeper.Update(repoName, repo)

	return nil
}

func makeProposal(
	spk *crypto.PubKey,
	repo *state.Repository,
	id string,
	proposalFee util.String,
	chainHeight uint64) *state.RepoProposal {

	proposal := &state.RepoProposal{
		ID:         id,
		Config:     repo.Config.Clone().Governance,
		Creator:    spk.Addr().String(),
		Height:     chainHeight,
		EndAt:      repo.Config.Governance.DurOfProposal + chainHeight + 1,
		Fees:       map[string]string{},
		ActionData: map[string][]byte{},
	}

	// Register proposal fee if set
	if proposalFee != "0" {
		proposal.Fees.Add(spk.Addr().String(), proposalFee.String())
	}

	// Set the max. join height for voters.
	if repo.Config.Governance.VoterAgeAsCurHeight {
		proposal.ProposerMaxJoinHeight = chainHeight + 1
	}

	// Set the fee deposit end height and also update the proposal end height to
	// be after the fee deposit height
	if repo.Config.Governance.FeeDepositDurOfProposal > 0 {
		proposal.FeeDepositEndAt = 1 + chainHeight + repo.Config.Governance.FeeDepositDurOfProposal
		proposal.EndAt = proposal.FeeDepositEndAt + repo.Config.Governance.DurOfProposal
	}

	// Register the proposal to the repo
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
// proposalFee: The proposal anti-spam fee
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
	repo := repoKeeper.Get(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = core.TxTypeRepoProposalMergeRequest
	proposal.ActionData = map[string][]byte{
		constants.ActionDataKeyBaseBranch:   util.ToBytes(baseBranch),
		constants.ActionDataKeyBaseHash:     util.ToBytes(baseBranchHash),
		constants.ActionDataKeyTargetBranch: util.ToBytes(targetBranch),
		constants.ActionDataKeyTargetHash:   util.ToBytes(targetBranchHash),
	}

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.debitAccount(spk, totalFee, chainHeight)

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

// applyProposalRegisterPushKeys updates a repo's contributor collection.
func applyProposalRegisterPushKeys(
	keepers core.Keepers,
	proposal state.Proposal,
	repo *state.Repository,
	chainHeight uint64) error {

	ad := proposal.GetActionData()

	// Extract the policies.
	var policies []*state.ContributorPolicy
	_ = util.ToObject(ad[constants.ActionDataKeyPolicies], &policies)

	// Extract the push key IDs.
	var pushKeyIDs []string
	_ = util.ToObject(ad[constants.ActionDataKeyIDs], &pushKeyIDs)

	// Extract fee mode and fee cap
	var feeMode state.FeeMode
	_ = util.ToObject(ad[constants.ActionDataKeyFeeMode], &feeMode)
	var feeCap = util.String("0")
	if feeMode == state.FeeModeRepoPaysCapped {
		_ = util.ToObject(ad[constants.ActionDataKeyFeeCap], &feeCap)
	}

	// Get any target namespace.
	var namespace, namespaceOnly, targetNS string
	var ns *state.Namespace
	if _, ok := ad[constants.ActionDataKeyNamespace]; ok {
		util.ToObject(ad[constants.ActionDataKeyNamespace], &namespace)
		targetNS = namespace
	}
	if _, ok := ad[constants.ActionDataKeyNamespaceOnly]; ok {
		util.ToObject(ad[constants.ActionDataKeyNamespaceOnly], &namespaceOnly)
		targetNS = namespaceOnly
	}
	if targetNS != "" {
		ns = keepers.NamespaceKeeper().Get(util.HashNamespace(targetNS))
		if ns.IsNil() {
			panic("namespace must exist")
		}
	}

	// For each push key ID, add a contributor.
	// This will replace any existing contributor with matching push key ID.
	for _, pushKeyID := range pushKeyIDs {

		contributor := &state.BaseContributor{FeeCap: feeCap, FeeUsed: "0", Policies: policies}

		// If namespace is set, add the contributor to the the namespace and
		// then if namespaceOnly is set, continue  to the next push key
		// id after adding a contributor to the namespace
		if namespace != "" || namespaceOnly != "" {
			ns.Contributors[pushKeyID] = contributor
			if namespaceOnly != "" {
				continue
			}
		}

		// Register contributor to the repo
		repo.Contributors[pushKeyID] = &state.RepoContributor{
			FeeMode:  feeMode,
			FeeCap:   contributor.FeeCap,
			FeeUsed:  contributor.FeeUsed,
			Policies: contributor.Policies,
		}
	}

	if ns != nil {
		keepers.NamespaceKeeper().Update(util.HashNamespace(targetNS), ns)
	}

	return nil
}

// execRepoProposalRegisterPushKeys creates a proposal to register one or more push key ID as contributors
//
// ARGS:
// senderPubKey: The public key of the transaction sender.
// repoName: The name of the target repository.
// pushKeyIDs: The list of push key IDs to register
// feeMode: The fee mode for the push key IDs
// feeCap: The max fee the push key IDs can spend
// aclPolicies: Access control policies for the push key IDs
// namespace: A namespace that the push key IDs will also be registered to.
// namespaceOnly: Like 'namespace' but the push key IDs will not be registered to the repo.
// proposalFee: The proposal anti-spam fee
// fee: The fee to be paid by the sender.
// chainHeight: The height of the block chain
//
// CONTRACT: Sender's public key must be valid
func (t *Transaction) execRepoProposalRegisterPushKeys(
	senderPubKey util.Bytes32,
	repoName,
	proposalID string,
	pushKeyIDs []string,
	feeMode state.FeeMode,
	feeCap util.String,
	aclPolicies []*state.ContributorPolicy,
	namespace string,
	namespaceOnly string,
	proposalFee,
	fee util.String,
	chainHeight uint64) error {

	// Get the repo
	repoKeeper := t.logic.RepoKeeper()
	repo := repoKeeper.Get(repoName)

	// Create a proposal
	spk, _ := crypto.PubKeyFromBytes(senderPubKey.Bytes())
	proposal := makeProposal(spk, repo, proposalID, proposalFee, chainHeight)
	proposal.Action = core.TxTypeRepoProposalRegisterPushKey
	proposal.ActionData = map[string][]byte{
		constants.ActionDataKeyIDs:      util.ToBytes(pushKeyIDs),
		constants.ActionDataKeyPolicies: util.ToBytes(aclPolicies),
		constants.ActionDataKeyFeeMode:  util.ToBytes(feeMode),
		constants.ActionDataKeyFeeCap:   util.ToBytes(feeCap.String()),
	}
	if namespace != "" {
		proposal.ActionData[constants.ActionDataKeyNamespace] = util.ToBytes(namespace)
	}
	if namespaceOnly != "" {
		proposal.ActionData[constants.ActionDataKeyNamespaceOnly] = util.ToBytes(namespaceOnly)
	}

	// Deduct network fee + proposal fee from sender
	totalFee := fee.Decimal().Add(proposalFee.Decimal())
	t.debitAccount(spk, totalFee, chainHeight)

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
	repo := repoKeeper.Get(repoName)
	prop := repo.Proposals.Get(proposalID)

	increments := float64(0)

	// When proposers are the owners, and tally method is ProposalTallyMethodIdentity
	// each proposer will have 1 voting power.
	if prop.Config.Voter == state.VoteByOwner &&
		prop.Config.TallyMethodOfProposal == state.ProposalTallyMethodIdentity {
		increments = 1
	}

	// When proposers are the owners, and tally method is ProposalTallyMethodCoinWeighted
	// each proposer will use the value of the voter's spendable account balance
	// as their voting power.
	if prop.Config.Voter == state.VoteByOwner &&
		prop.Config.TallyMethodOfProposal == state.ProposalTallyMethodCoinWeighted {
		senderAcct := t.logic.AccountKeeper().Get(spk.Addr())
		increments = senderAcct.GetSpendableBalance(chainHeight).Float()
	}

	// For network staked-weighted votes, use the total value of coins directly
	// staked by the voter as their vote power
	if prop.Config.TallyMethodOfProposal == state.ProposalTallyMethodNetStakeOfProposer {
		increments, err = t.logic.GetTicketManager().
			ValueOfNonDelegatedTickets(senderPubKey, prop.ProposerMaxJoinHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get value of non-delegated tickets of sender")
		}
	}

	// For network staked-weighted votes, use the total value of coins delegated
	// to the voter as their vote power
	if prop.Config.TallyMethodOfProposal == state.ProposalTallyMethodNetStakeOfDelegators {
		increments, err = t.logic.GetTicketManager().
			ValueOfDelegatedTickets(senderPubKey, prop.ProposerMaxJoinHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get value of delegated tickets of sender")
		}
	}

	// For network staked-weighted votes, use the total value of coins delegated
	// to the voter as their vote power
	if prop.Config.TallyMethodOfProposal == state.ProposalTallyMethodNetStake {

		tickets, err := t.logic.GetTicketManager().
			GetNonDecayedTickets(senderPubKey, prop.ProposerMaxJoinHeight)
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
				case state.ProposalVoteYes:
					newYes := decimal.NewFromFloat(prop.Yes)
					newYes = newYes.Sub(ticket.Value.Decimal())
					prop.Yes, _ = newYes.Float64()

				case state.ProposalVoteNo:
					newNo := decimal.NewFromFloat(prop.No)
					newNo = newNo.Sub(ticket.Value.Decimal())
					prop.Yes, _ = newNo.Float64()

				case state.ProposalVoteNoWithVeto:
					newNoWithVeto := decimal.NewFromFloat(prop.NoWithVeto)
					newNoWithVeto = newNoWithVeto.Sub(ticket.Value.Decimal())
					prop.NoWithVeto, _ = newNoWithVeto.Float64()

				case state.ProposalVoteAbstain:
					newAbstain := decimal.NewFromFloat(prop.Abstain)
					newAbstain = newAbstain.Sub(ticket.Value.Decimal())
					prop.Abstain, _ = newAbstain.Float64()
				}

				sumValue = sumValue.Add(ticket.Value.Decimal())
			}
		}

		increments, _ = sumValue.Float64()
	}

	if vote == state.ProposalVoteYes {
		prop.Yes += increments
	} else if vote == state.ProposalVoteNo {
		prop.No += increments
	} else if vote == state.ProposalVoteAbstain {
		prop.Abstain += increments
	} else if vote == state.ProposalVoteNoWithVeto {
		prop.NoWithVeto += increments

		// Also, if the proposer type for the proposal is stakeholders and veto
		// owners and voter is an owner, increment NoWithVetoByOwners by 1
		voterOwnerObj := repo.Owners.Get(spk.Addr().String())
		isStakeholderAndVetoOwnerProposer := prop.Config.Voter == state.VoteByNetStakersAndVetoOwner
		if isStakeholderAndVetoOwnerProposer && voterOwnerObj != nil && voterOwnerObj.Veto {
			prop.NoWithVetoByOwners = 1
		}
	}

	// Update the repo
	repoKeeper.Update(repoName, repo)

	// Deduct fee from sender
	t.debitAccount(spk, fee.Decimal(), chainHeight)

	return nil
}
