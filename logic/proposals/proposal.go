package proposals

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
	"github.com/themakeos/lobe/params"
	tickettypes "github.com/themakeos/lobe/ticket/types"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/util"
	"github.com/themakeos/lobe/util/identifier"
	"github.com/thoas/go-funk"
)

func MakeProposal(
	creatorAddress string,
	repo *state.Repository,
	id string,
	proposalFee util.String,
	chainHeight uint64) *state.RepoProposal {

	proposal := &state.RepoProposal{
		ID:         id,
		Config:     repo.Config.Clone().Gov,
		Creator:    creatorAddress,
		Height:     util.UInt64(chainHeight),
		EndAt:      repo.Config.Gov.PropDuration + util.UInt64(chainHeight) + 1,
		Fees:       map[string]string{},
		ActionData: map[string]util.Bytes{},
	}

	// Register proposal fee if set
	if proposalFee != "0" {
		proposal.Fees.Add(creatorAddress, proposalFee.String())
	}

	// Set the max. join height for voters.
	if repo.Config.Gov.ReqVoterJoinHeight {
		proposal.ProposerMaxJoinHeight = util.UInt64(chainHeight) + 1
	}

	// Set the fee deposit end height and also update the proposal end height to
	// be after the fee deposit height
	if repo.Config.Gov.PropFeeDepositDur > 0 {
		proposal.FeeDepositEndAt = 1 + util.UInt64(chainHeight) + repo.Config.Gov.PropFeeDepositDur
		proposal.EndAt = proposal.FeeDepositEndAt + repo.Config.Gov.PropDuration
	}

	// Register the proposal to the repo
	repo.Proposals.Add(proposal.ID, proposal)

	return proposal
}

// GetProposalOutcome returns the current outcome of a proposal
// whose voters are only network stakeholders; If the proposal requires
// a proposer max join height, only stakeholders whose tickets became mature
// before the proposer max join height
func GetProposalOutcome(tickmgr tickettypes.TicketManager, prop state.Proposal,
	repo *state.Repository) state.ProposalOutcome {

	var err error
	totalPower := float64(0)

	// When proposers are only the owners of the repo, the power is the total
	// number of owners of the repository - one vote to one owner.
	// However, If there is a max proposer join height, eligible owners are
	// those who joined on or before the proposer max join height.
	if prop.GetVoterType() == state.VoterOwner {
		maxJoinHeight := prop.GetVoterMaxJoinHeight()
		repo.Owners.ForEach(func(o *state.RepoOwner, addr string) {
			if maxJoinHeight > 0 && maxJoinHeight < o.JoinedAt.UInt64() {
				return
			}
			totalPower++
		})
	}

	// When proposers include only network stakeholders, the total power is the total
	// value of mature and active tickets on the network.
	if prop.GetVoterType() == state.VoterNetStakers ||
		prop.GetVoterType() == state.VoterNetStakersAndVetoOwner {
		totalPower, err = tickmgr.ValueOfAllTickets(prop.GetVoterMaxJoinHeight())
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
	// the vote iff the "NoWithVetoByOwners" reached the veto owner quorum.
	if prop.GetVoterType() == state.VoterNetStakersAndVetoOwner {
		if nRejectedWithVetoVotesByOwners > 0 && nRejectedWithVetoVotesByOwners >= vetoOwnerQuorum {
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

// DetermineProposalOutcome determines the outcome of the proposal votes
func DetermineProposalOutcome(
	keepers core.Keepers,
	proposal state.Proposal,
	repo *state.Repository,
	chainHeight uint64) state.ProposalOutcome {
	return GetProposalOutcome(keepers.GetTicketManager(), proposal, repo)
}

// refundProposalFees refunds all fees back to their senders
func refundProposalFees(keepers core.Keepers, proposal state.Proposal) error {
	for senderAddr, fee := range proposal.GetFees() {
		sender := identifier.Address(senderAddr)
		acct := keepers.AccountKeeper().Get(sender)
		acct.Balance = util.String(acct.Balance.Decimal().Add(util.String(fee).Decimal()).String())
		keepers.AccountKeeper().Update(sender, acct)
	}
	return nil
}

// MaybeProcessProposalFee determines and execute proposal fee refund or distribution
func MaybeProcessProposalFee(
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
		expected := []state.ProposalOutcome{
			state.ProposalOutcomeBelowThreshold,
			state.ProposalOutcomeAccepted,
		}
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

type ApplyProposalArgs struct {
	Keepers     core.Keepers
	Proposal    state.Proposal
	Repo        *state.Repository
	ChainHeight uint64
	Contracts   []core.SystemContract
}

// MaybeApplyProposal attempts to apply the action of a proposal
func MaybeApplyProposal(args *ApplyProposalArgs) (bool, error) {

	// When the proposal has already been finalized, do nothing.
	if args.Proposal.IsFinalized() {
		return false, nil
	}

	// When the proposal has fee deposit enabled and the deposit period has
	// passed, but not enough deposits where paid, there will be no votes as
	// such we move return any existing deposits to their senders and set
	// the outcome.
	if args.Proposal.IsFeeDepositEnabled() &&
		!args.Proposal.IsDepositPeriod(args.ChainHeight+1) &&
		!args.Proposal.IsDepositedFeeOK() {
		args.Proposal.SetOutcome(state.ProposalOutcomeInsufficientDeposit)
		return false, refundProposalFees(args.Keepers, args.Proposal)
	}

	var err error
	var outcome state.ProposalOutcome

	// When allowed voters are only the repo owners and there is just one owner
	// whom is also the creator of the proposal, instantly apply the args.Proposal.
	isOwnersOnlyProposal := args.Proposal.GetVoterType() == state.VoterOwner
	if isOwnersOnlyProposal && len(args.Repo.Owners) == 1 && args.Repo.Owners.Has(args.Proposal.GetCreator()) {
		outcome = state.ProposalOutcomeAccepted
		args.Proposal.SetOutcome(outcome)
		args.Proposal.IncrAccept()
		goto apply
	}

	// Don't apply the proposal if the proposal end height is in the future.
	if args.Proposal.GetEndAt() > args.ChainHeight+1 {
		return false, nil
	}

	// Here, the proposal has come to its end. We need to determine if the
	// outcome was an acceptance, if not we return false.
	outcome = DetermineProposalOutcome(args.Keepers, args.Proposal, args.Repo, args.ChainHeight)
	args.Proposal.SetOutcome(outcome)
	if outcome != state.ProposalOutcomeAccepted {
		err := MaybeProcessProposalFee(outcome, args.Keepers, args.Proposal, args.Repo)
		return false, err
	}

apply:

	for _, contract := range args.Contracts {
		if !contract.CanExec(args.Proposal.GetAction()) {
			continue
		}
		err = contract.(core.ProposalContract).Apply(&core.ProposalApplyArgs{
			Proposal:    args.Proposal,
			Repo:        args.Repo,
			Keepers:     args.Keepers,
			ChainHeight: args.ChainHeight,
		})
		if err != nil {
			return false, err
		}
		break
	}

	if err = MaybeProcessProposalFee(outcome, args.Keepers, args.Proposal, args.Repo); err != nil {
		return false, err
	}

	return true, nil
}
