package issues

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"
)

// AddIssueOrCommentCommitCmd creates a new issue or adds a comment commit to an existing issue.
// Returns true if target issue was newly created.
// The target reference is also returned
func AddIssueOrCommentCommitCmd(targetRepo core.BareRepo, targetIssue, issueBody string) (bool, string, error) {

	var issueRefName string
	if targetIssue != "" {
		issueRefName = fmt.Sprintf("refs/heads/issues/%s", targetIssue)
	} else {
		issueBranchName := fmt.Sprintf("%d-%s", time.Now().Unix(), plumbing.MakeCommitHash(issueBody))
		issueRefName = fmt.Sprintf("refs/heads/issues/%s", issueBranchName)
	}

	// Check if the issue exist
	hash, err := targetRepo.RefGet(issueRefName)
	if err != nil {
		if err != repo2.ErrRefNotFound {
			return false, "", errors.Wrap(err, "failed to check issue existence")
		}
	}
	if hash == "" && targetIssue != "" {
		return false, "", fmt.Errorf("issue (%s) does not exist", targetIssue)
	}

	// Create an issue commit (pass the current reference hash as parent)
	issueHash, err := targetRepo.CreateSingleFileCommit("body", issueBody, hash)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to create root issue commit")
	}

	// Update the current hash of the issue reference
	if err = targetRepo.RefUpdate(issueRefName, issueHash); err != nil {
		return false, "", errors.Wrap(err, "failed to update issue branch")
	}

	return hash == "", issueRefName, nil
}
