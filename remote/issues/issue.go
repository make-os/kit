package issues

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"
)

// MustGetUnusedIssueID finds an issue ID that has not been used locally
func MustGetUnusedIssueID(targetRepo core.BareRepo, startID int) int {
	for {
		ref := plumbing.MakeIssueReference(startID)
		hash, err := targetRepo.RefGet(ref)
		if err != nil && err != repo.ErrRefNotFound {
			panic(err)
		}
		if hash == "" {
			return startID
		}
		startID++
	}
}

// AddIssueOrCommentCommit creates a new issue or adds a comment commit to an existing issue.
// Returns true if target issue was newly created.
// The target reference is also returned
func AddIssueOrCommentCommit(
	targetRepo core.BareRepo,
	issueID int,
	issueBody string,
	isComment bool) (bool, string, error) {

	var err error
	var ref string

	if issueID != 0 {
		ref = plumbing.MakeIssueReference(issueID)
	}

	// When an issue ID is not provided, incrementally find an unused issue ID
	// starting with the (num. of existing issue + 1)
	if issueID == 0 {
		issueID, err = targetRepo.NumIssueBranches()
		if err != nil {
			return false, "", errors.Wrap(err, "failed to get number of issues")
		}
		issueID++
		issueID = MustGetUnusedIssueID(targetRepo, issueID)
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
		return false, "", fmt.Errorf("issue (%d) does not exist", issueID)
	}

	// Create an issue commit (pass the current reference hash as parent)
	issueHash, err := targetRepo.CreateSingleFileCommit("body", issueBody, "", hash)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to create root issue commit")
	}

	// Update the current hash of the issue reference
	if err = targetRepo.RefUpdate(ref, issueHash); err != nil {
		return false, "", errors.Wrap(err, "failed to update issue branch")
	}

	return hash == "", ref, nil
}

// MakeIssueBody creates an issue body using the specified fields
func MakeIssueBody(title, body string, replyTo int, labels, assignees, fixers []string) string {
	args := ""
	str := "---\n%s---\n"

	if len(title) > 0 {
		args += fmt.Sprintf("title: %s\n", title)
	}
	if replyTo > 0 {
		args += fmt.Sprintf("replyTo: %d\n", replyTo)
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
