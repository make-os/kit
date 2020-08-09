package validation

import (
	"fmt"

	"github.com/themakeos/lobe/logic/contracts/mergerequest"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type MergeComplianceCheckFunc func(
	repo types.LocalRepo,
	change *types.ItemChange,
	mergeProposalID,
	pushKeyID string,
	keepers core.Keepers) error

// CheckMergeCompliance checks whether the change satisfies the given merge proposal
func CheckMergeCompliance(
	repo types.LocalRepo,
	change *types.ItemChange,
	mergeProposalID, pushKeyID string,
	keepers core.Keepers) error {

	ref := plumbing.ReferenceName(change.Item.GetName())
	if !ref.IsBranch() {
		return fmt.Errorf("merge error: pushed reference must be a branch")
	}

	propID := mergerequest.MakeMergeRequestProposalID(mergeProposalID)
	prop := repo.GetState().Proposals.Get(propID)
	if prop == nil {
		return fmt.Errorf("merge error: target merge proposal was not found")
	}

	// Ensure the signer is the creator of the proposal
	pushKey := keepers.PushKeyKeeper().Get(pushKeyID)
	if pushKey.Address.String() != prop.Creator {
		return fmt.Errorf("merge error: push key owner did not create the proposal")
	}

	// Check if the merge proposal has been closed
	closed, err := keepers.RepoKeeper().IsProposalClosed(repo.GetName(), propID)
	if err != nil {
		return fmt.Errorf("merge error: %s", err)
	} else if closed {
		return fmt.Errorf("merge error: target merge proposal is already closed")
	}

	// Ensure the proposal's base branch matches the pushed branch
	var propBaseBranch = string(prop.ActionData[constants.ActionDataKeyBaseBranch])
	if ref.Short() != propBaseBranch {
		return fmt.Errorf("merge error: pushed branch name and proposal base branch name must match")
	}

	// Check whether the merge proposal has been accepted
	if !prop.IsAccepted() {
		if prop.Outcome == 0 {
			return fmt.Errorf("merge error: target merge proposal is undecided")
		} else {
			return fmt.Errorf("merge error: target merge proposal was not accepted")
		}
	}

	// Ensure the new commit and the proposal target match
	var propTargetHash = string(prop.ActionData[constants.ActionDataKeyTargetHash])
	if change.Item.GetData() != propTargetHash {
		return fmt.Errorf("merge error: pushed commit did not match merge proposal target hash")
	}

	return nil
}
