package repo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/makeos/mosdef/types"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// merge takes a base branch merges target branch into it.
// target can be of the form '<remote-repo>:branch>' to represent a target
// branch another repo.
// TODO: Remove if not in use
func merge(baseRepo types.BareRepo, base, target, reposDir string, uncommitted bool) error {

	// Check if repo has the base branch
	_, err := baseRepo.Reference(plumbing.NewBranchReferenceName(base), false)
	if err != nil {
		return errors.Wrap(err, "failed to find base branch")
	}

	targetRepoName := ""
	targetBranchName := target
	if strings.Index(target, ":") != -1 {
		targetParts := strings.Split(target, ":")
		if len(targetParts) != 2 {
			return fmt.Errorf("invalid target format")
		}
		targetRepoName = targetParts[0]
		targetBranchName = targetParts[1]
	}

	// If no target repo, we expect the branch to be in the base repo.
	targetRepoPath := ""
	if targetRepoName == "" {
		_, err = baseRepo.Reference(plumbing.NewBranchReferenceName(targetBranchName), false)
		if err != nil {
			return errors.Wrap(err, "failed to find target branch")
		}
	} else {
		targetRepoPath = filepath.Join(reposDir, targetRepoName)
		targetRepo, err := GetRepo(targetRepoPath)
		if err != nil {
			return errors.Wrap(err, "failed to find target branch's repo")
		}

		// Ensure target branch exist in the repo
		_, err = targetRepo.Reference(plumbing.NewBranchReferenceName(targetBranchName), false)
		if err != nil {
			return errors.Wrap(err, "failed to find target branch")
		}
	}

	if uncommitted {
		err = baseRepo.TryMergeBranch(base, targetBranchName, targetRepoPath)
	} else {
		err = baseRepo.MergeBranch(base, targetBranchName, targetRepoPath)
	}

	return errors.Wrap(err, "merge operation failed")
}
