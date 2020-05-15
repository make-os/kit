package issues

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"
)

// IssueCommentCreator is a function type for creating an issue or adding comments to an issue
type IssueCommentCreator func(targetRepo core.LocalRepo,
	issueID int,
	issueBody string,
	isComment bool) (isNewIssue bool, issueReference string, err error)

// CreateIssueComment creates a new issue and/or adds a comment commit to a new/existing issue.
func CreateIssueComment(
	targetRepo core.LocalRepo,
	issueID int,
	issueBody string,
	isComment bool) (isNewIssue bool, issueReference string, err error) {

	var ref string

	if issueID != 0 {
		ref = plumbing.MakeIssueReference(issueID)
	}

	// When an issue number is not provided, find an unused number monotonically
	if issueID == 0 {
		issueID, err = targetRepo.GetFreeIssueNum(1)
		if err != nil {
			return false, "", errors.Wrap(err, "failed to find free issue number")
		}
		ref = plumbing.MakeIssueReference(issueID)
	}

	// Check if the issue reference already exist exist
	hash, err := targetRepo.RefGet(ref)
	if err != nil {
		if err != repo.ErrRefNotFound {
			return false, "", errors.Wrap(err, "failed to check issue existence")
		}
	}

	// To create comment commit, the issue must already exist
	if hash == "" && isComment {
		return false, "", fmt.Errorf("can't add comment to a non-existing issue (%d)", issueID)
	}

	// Create an issue commit (pass the current reference hash as parent)
	issueHash, err := targetRepo.CreateSingleFileCommit("body", issueBody, "", hash)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to create issue commit")
	}

	// Update the current hash of the issue reference
	if err = targetRepo.RefUpdate(ref, issueHash); err != nil {
		return false, "", errors.Wrap(err, "failed to update issue reference target hash")
	}

	return hash == "", ref, nil
}
