package logic

import "github.com/makeos/mosdef/types"

import "fmt"

// canApplyAction checks whether the proposal is at
// a state where the action can be applied.
func canApplyAction(proposal types.Proposal, repo *types.Repository) bool {

	if proposal.IsFinalized() {
		return false
	}

	// For proposals were allowed participants are only the repo owners, return true when
	// there is just only one owner who is also to creator of the proposal
	if proposal.GetProposeeType() == types.ProposeeOwner &&
		len(repo.Owners) == 1 && repo.Owners.Has(proposal.GetCreator()) {
		return true
	}

	return false
}

// maybeApplyProposal attempts to apply the action of a proposal
func maybeApplyProposal(
	keepers types.Keepers,
	proposal types.Proposal,
	repo *types.Repository,
	chainHeight uint64) (bool, error) {

	// Check if we can apply the action of the proposal
	if !canApplyAction(proposal, repo) {
		return false, nil
	}

	switch proposal.GetAction() {
	case types.ProposalActionAddOwner:
		return true, applyAddOwnerProposal(proposal, repo, chainHeight)
	}

	return false, fmt.Errorf("unsupported proposal action")
}
