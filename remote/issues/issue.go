package issues

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"
)

// IssueCommentCreator is a function type for creating an issue or adding comments to an issue
type IssueCommentCreator func(targetRepo core.BareRepo,
	issueID int,
	issueBody string,
	isComment bool) (isNewIssue bool, issueReference string, err error)

// CreateIssueComment creates a new issue and/or adds a comment commit to a new/existing issue.
func CreateIssueComment(
	targetRepo core.BareRepo,
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

// MakeIssueBody creates an issue body using the specified fields
func MakeIssueBody(
	title,
	body,
	replyTo string,
	reactions,
	labels,
	assignees,
	fixers []string) string {

	args := ""
	str := "---\n%s---\n"

	if len(title) > 0 {
		args += fmt.Sprintf("title: %s\n", title)
	}
	if replyTo != "" {
		args += fmt.Sprintf("replyTo: %s\n", replyTo)
	}
	if len(reactions) > 0 {
		reactionsStr, _ := json.Marshal(reactions)
		args += fmt.Sprintf("reactions: %s\n", reactionsStr)
	}
	if len(labels) > 0 {
		labelsStr, _ := json.Marshal(labels)
		args += fmt.Sprintf("labels: %s\n", labelsStr)
	}
	if len(assignees) > 0 {
		assigneesStr, _ := json.Marshal(assignees)
		args += fmt.Sprintf("assignees: %s\n", assigneesStr)
	}
	if len(fixers) > 0 {
		fixersStr, _ := json.Marshal(fixers)
		args += fmt.Sprintf("fixers: %s\n", fixersStr)
	}

	return fmt.Sprintf(str, args) + body
}
