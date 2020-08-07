package validation

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/themakeos/lobe/logic/contracts/mergerequest"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type MergeComplianceCheckFunc func(
	repo types.LocalRepo,
	change *types.ItemChange,
	oldRef types.Item,
	mergeProposalID,
	pushKeyID string,
	keepers core.Keepers) error

// CheckMergeCompliance checks whether push to a branch satisfied an accepted merge proposal
func CheckMergeCompliance(
	repo types.LocalRepo,
	change *types.ItemChange,
	oldRef types.Item,
	mergeProposalID,
	pushKeyID string,
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

	// Get the commit that initiated the merge operation (a.k.a "pushed commit").
	// Since by convention, its parent is considered the actual merge target.
	// As such, we need to perform some validation before we compare it with
	// the merge proposal target hash.
	commit, err := repo.WrappedCommitObject(plumbing.NewHash(change.Item.GetData()))
	if err != nil {
		return errors.Wrap(err, "unable to get commit object")
	}

	// By default, the parent of the merge commit is target commit...
	targetCommit, _ := commit.Parent(0)

	// ...unless the merge commit is the proposal target, in which case
	// we use the commit as the target hash.
	var propTargetHash = string(prop.ActionData[constants.ActionDataKeyTargetHash])
	if propTargetHash == commit.GetHash().String() {
		targetCommit = commit
	}

	// When the merge commit has parents, ensure the proposal target is a parent.
	// Extract it and use as the target commit.
	if commit.NumParents() > 1 {
		_, targetCommit = commit.IsParent(propTargetHash)
		if targetCommit == nil {
			return fmt.Errorf("merge error: target hash is not a parent of the merge commit")
		}
	}

	// Ensure the difference between the target commit and the pushed commit
	// only exist in the commit hash and not the tree, author and committer information.
	// By convention, the pushed commit can only modify its commit object (time,
	// message and signature).
	if commit.GetTreeHash() != targetCommit.GetTreeHash() ||
		commit.GetAuthor().String() != targetCommit.GetAuthor().String() ||
		commit.GetCommitter().String() != targetCommit.GetCommitter().String() {
		return fmt.Errorf("merge error: pushed commit must not modify target branch history")
	}

	// When no older reference (ex. a new/first branch),
	// set default hash value to zero hash.
	oldRefHash := plumbing.ZeroHash.String()
	if oldRef != nil {
		oldRefHash = oldRef.GetData()
	}

	// When no base hash is given, set default hash value to zero hash
	var propBaseHash = string(prop.ActionData[constants.ActionDataKeyBaseHash])
	propBaseHashStr := plumbing.ZeroHash.String()
	if propBaseHash != "" {
		propBaseHashStr = propBaseHash
	}

	// Ensure the proposals base branch hash matches the hash of the current
	// branch before this current push/change.
	if propBaseHashStr != oldRefHash {
		return fmt.Errorf("merge error: target merge proposal base branch hash is stale or invalid")
	}

	// Ensure the target commit and the proposal target match
	if targetCommit.GetHash().String() != propTargetHash {
		return fmt.Errorf("merge error: target commit hash and the merge proposal target hash must match")
	}

	return nil
}
