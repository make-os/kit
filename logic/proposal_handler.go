package logic

import (
	"fmt"
	"math"

	types3 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"
)

// getProposalOutcome returns the current outcome of a proposal
// whose voters are only network stakeholders; If the proposal requires
// a proposer max join height, only stakeholders whose tickets became mature
// before the proposer max join height
func getProposalOutcome(
	tickmgr types3.TicketManager,
	prop state.Proposal,
	repo *state.Repository) state.ProposalOutcome {

	var err error
	totalPower := float64(0)

	// When proposers are only the owners of the repo, the power is the total
	// number of owners of the repository - one vote to one owner.
	// However, If there is a max proposer join height, eligible owners are
	// those who joined on or before the proposer max join height.
	if prop.GetProposerType() == state.VoteByOwner {
		maxJoinHeight := prop.GetProposerMaxJoinHeight()
		repo.Owners.ForEach(func(o *state.RepoOwner, addr string) {
			if maxJoinHeight > 0 && maxJoinHeight < o.JoinedAt {
				return
			}
			totalPower++
		})
	}

	// When proposers include only network stakeholders, the total power is the total
	// value of mature and active tickets on the network.
	if prop.GetProposerType() == state.VoteByNetStakers ||
		prop.GetProposerType() == state.VoteByNetStakersAndVetoOwner {
		totalPower, err = tickmgr.ValueOfAllTickets(prop.GetProposerMaxJoinHeight())
		if err != nil {
			panic(err)
		}
	}

	nAcceptedVotes := prop.GetAccepted()
	nRejectedVotes := prop.GetRejected()
	nRejectedWithVetoVotes := prop.GetRejectedWithVeto()
	nRejectedWithVetoVotesByOwners := prop.GetRejectedWithVetoByOwners()
	numOwners := float64(len(repo.Owners))
	totalVotesReceived := nAcceptedVotes + nRejectedVotes + nRejectedWithVetoVotes
	quorum := math.Round(totalPower * (prop.GetQuorum() / 100))
	threshold := math.Round(totalVotesReceived * (prop.GetThreshold() / 100))
	vetoQuorum := math.Round(totalVotesReceived * (prop.GetVetoQuorum() / 100))
	vetoOwnerQuorum := math.Round(numOwners * (prop.GetVetoOwnersQuorum() / 100))

	// Check if general vote quorum is satisfied.
	// Ensures that a certain number of general vote population participated.
	if totalVotesReceived < quorum {
		return state.ProposalOutcomeQuorumNotMet
	}

	// Check if the "NoWithVeto" votes reached the general veto quorum.
	if nRejectedWithVetoVotes > 0 && nRejectedWithVetoVotes >= vetoQuorum {
		return state.ProposalOutcomeRejectedWithVeto
	}

	// When proposer are stakeholders and veto owners, the veto owners win
	// the vote iff the "NoWithVetoByOwners" reached the special veto owner quorum.
	if prop.GetProposerType() == state.VoteByNetStakersAndVetoOwner {
		if nRejectedWithVetoVotesByOwners > 0 &&
			nRejectedWithVetoVotesByOwners >= vetoOwnerQuorum {
			return state.ProposalOutcomeRejectedWithVetoByOwners
		}
	}

	accepted := nAcceptedVotes >= threshold
	rejected := nRejectedVotes >= threshold

	// Check if the "Yes" votes reached the threshold
	if accepted && !rejected {
		return state.ProposalOutcomeAccepted
	}

	// Check if the "No" votes reached the threshold
	if rejected && !accepted {
		return state.ProposalOutcomeRejected
	}

	return state.ProposalOutcomeBelowThreshold
}

// determineProposalOutcome determines the outcome of the proposal votes
func determineProposalOutcome(
	keepers core.Keepers,
	proposal state.Proposal,
	repo *state.Repository,
	chainHeight uint64) state.ProposalOutcome {
	return getProposalOutcome(keepers.GetTicketManager(), proposal, repo)
}

// refundProposalFees refunds all fees back to their senders
func refundProposalFees(keepers core.Keepers, proposal state.Proposal) error {
	for senderAddr, fee := range proposal.GetFees() {
		sender := util.Address(senderAddr)
		acct := keepers.AccountKeeper().Get(sender)
		acct.Balance = util.String(acct.Balance.Decimal().Add(util.String(fee).Decimal()).String())
		keepers.AccountKeeper().Update(sender, acct)
	}
	return nil
}

// maybeProcessProposalFee determines and execute
// proposal fee refund or distribution
func maybeProcessProposalFee(
	outcome state.ProposalOutcome,
	keepers core.Keepers,
	proposal state.Proposal,
	repo *state.Repository) error {

	switch proposal.GetRefundType() {
	case state.ProposalFeeRefundNo:
		goto dist
	case state.ProposalFeeRefundOnAccept:
		if outcome == state.ProposalOutcomeAccepted {
			return refundProposalFees(keepers, proposal)
		}

	case state.ProposalFeeRefundOnAcceptReject:
		expected := []state.ProposalOutcome{state.ProposalOutcomeAccepted, state.ProposalOutcomeRejected}
		if funk.Contains(expected, outcome) {
			return refundProposalFees(keepers, proposal)
		}

	case state.ProposalFeeRefundOnAcceptAllReject:
		expected := []state.ProposalOutcome{
			state.ProposalOutcomeAccepted,
			state.ProposalOutcomeRejected,
			state.ProposalOutcomeRejectedWithVeto,
			state.ProposalOutcomeRejectedWithVetoByOwners,
		}
		if funk.Contains(expected, outcome) {
			return refundProposalFees(keepers, proposal)
		}

	case state.ProposalFeeRefundOnBelowThreshold:
		expected := []state.ProposalOutcome{state.ProposalOutcomeBelowThreshold}
		if funk.Contains(expected, outcome) {
			return refundProposalFees(keepers, proposal)
		}

	case state.ProposalFeeRefundOnBelowThresholdAccept:
		expected := []state.ProposalOutcome{state.ProposalOutcomeBelowThreshold, state.ProposalOutcomeAccepted}
		if funk.Contains(expected, outcome) {
			return refundProposalFees(keepers, proposal)
		}

	case state.ProposalFeeRefundOnBelowThresholdAcceptReject:
		expected := []state.ProposalOutcome{
			state.ProposalOutcomeBelowThreshold,
			state.ProposalOutcomeAccepted,
			state.ProposalOutcomeRejected,
		}
		if funk.Contains(expected, outcome) {
			return refundProposalFees(keepers, proposal)
		}

	case state.ProposalFeeRefundOnBelowThresholdAcceptAllReject:
		expected := []state.ProposalOutcome{
			state.ProposalOutcomeBelowThreshold,
			state.ProposalOutcomeAccepted,
			state.ProposalOutcomeRejected,
			state.ProposalOutcomeRejectedWithVeto,
			state.ProposalOutcomeRejectedWithVetoByOwners,
		}
		if funk.Contains(expected, outcome) {
			return refundProposalFees(keepers, proposal)
		}

	default:
		return fmt.Errorf("unknown proposal refund type")
	}

dist: // Distribute to repo and helm accounts
	totalFees := proposal.GetFees().Total()
	helmRepoName, _ := keepers.SysKeeper().GetHelmRepo()
	helmRepo := keepers.RepoKeeper().Get(helmRepoName)
	helmCut := decimal.NewFromFloat(params.HelmProposalFeeSplit).Mul(totalFees)
	helmRepo.SetBalance(helmRepo.Balance.Decimal().Add(helmCut).String())
	repoCut := decimal.NewFromFloat(params.TargetRepoProposalFeeSplit).Mul(totalFees)
	repo.SetBalance(repo.Balance.Decimal().Add(repoCut).String())
	keepers.RepoKeeper().Update(helmRepoName, helmRepo)

	return nil
}

// maybeApplyProposal attempts to apply the action of a proposal
func maybeApplyProposal(
	keepers core.Keepers,
	proposal state.Proposal,
	repo *state.Repository,
	chainHeight uint64) (bool, error) {

	// When the proposal has already been finalized, do nothing.
	if proposal.IsFinalized() {
		return false, nil
	}

	// When the proposal has fee deposit enabled and the deposit period has
	// passed, but not enough deposits where paid, there will be no votes as
	// such we move return any existing deposits to their senders and set
	// the outcome.
	if proposal.IsFeeDepositEnabled() &&
		!proposal.IsDepositPeriod(chainHeight+1) &&
		!proposal.IsDepositedFeeOK() {
		proposal.SetOutcome(state.ProposalOutcomeInsufficientDeposit)
		return false, refundProposalFees(keepers, proposal)
	}

	var err error
	var outcome state.ProposalOutcome

	// When allowed voters are only the repo owners and there is just one owner
	// whom is also the creator of the proposal, instantly apply the proposal.
	isOwnersOnlyProposal := proposal.GetProposerType() == state.VoteByOwner
	if isOwnersOnlyProposal && len(repo.Owners) == 1 && repo.Owners.Has(proposal.GetCreator()) {
		outcome = state.ProposalOutcomeAccepted
		proposal.SetOutcome(outcome)
		proposal.IncrAccept()
		goto apply
	}

	// Don't apply the proposal if the proposal end height is in the future.
	if proposal.GetEndAt() > chainHeight+1 {
		return false, nil
	}

	// Here, the proposal has come to its end. We need to determine if the
	// outcome was an acceptance, if not we return false.
	outcome = determineProposalOutcome(keepers, proposal, repo, chainHeight)
	proposal.SetOutcome(outcome)
	if outcome != state.ProposalOutcomeAccepted {
		err := maybeProcessProposalFee(outcome, keepers, proposal, repo)
		return false, err
	}

apply:
	switch proposal.GetAction() {
	case core.TxTypeRepoProposalUpsertOwner:
		err = applyProposalUpsertOwner(proposal, repo, chainHeight)
	case core.TxTypeRepoProposalUpdate:
		err = applyProposalRepoUpdate(proposal, repo, chainHeight)
	case core.TxTypeRepoProposalRegisterPushKey:
		err = applyProposalRegisterPushKeys(keepers, proposal, repo, chainHeight)
	case core.TxTypeRepoProposalMergeRequest:
	// Do nothing since there is no on-chain action
	default:
		err = fmt.Errorf("unsupported proposal action")
	}

	if err != nil {
		return false, err
	}

	if err := maybeProcessProposalFee(outcome, keepers, proposal, repo); err != nil {
		return false, err
	}

	return true, nil
}

// maybeApplyEndedProposals finds and applies proposals that will
// end at the given height.
func maybeApplyEndedProposals(
	keepers core.Keepers,
	nextChainHeight uint64) error {

	repoKeeper := keepers.RepoKeeper()

	// Get proposals ending at the given height
	endingProps := repoKeeper.GetProposalsEndingAt(nextChainHeight)

	// Attempt to apply and close the proposal
	for _, ep := range endingProps {
		repo := repoKeeper.Get(ep.RepoName)
		if repo.IsNil() {
			return fmt.Errorf("repo not found") // should never happen
		}
		_, err := maybeApplyProposal(keepers, repo.Proposals.Get(ep.ProposalID),
			repo, nextChainHeight-1)
		if err != nil {
			return err
		}
		repoKeeper.Update(ep.RepoName, repo)
	}

	return nil
}
