package issuecmd

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/pkg/errors"
)

// IssueCloseArgs contains parameters for IssueCloseCmd
type IssueCloseArgs struct {

	// Reference is the full reference path to the issue
	Reference string

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	// Force indicates that uncommitted changes should be ignored
	Force bool
}

type IssueCloseResult struct {
	Reference string
}

// IssueCloseCmdFunc describes IssueCloseCmd function signature
type IssueCloseCmdFunc func(r plumbing.LocalRepo, args *IssueCloseArgs) (*IssueCloseResult, error)

// IssueCloseCmd adds a close directive
func IssueCloseCmd(r plumbing.LocalRepo, args *IssueCloseArgs) (*IssueCloseResult, error) {

	// Ensure the issue reference exist
	recentCommentHash, err := r.RefGet(args.Reference)
	if err != nil {
		if err == plumbing.ErrRefNotFound {
			return nil, fmt.Errorf("issue not found")
		}
		return nil, err
	}

	pb, _, err := args.ReadPostBody(r, recentCommentHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read recent comment")
	} else if pointer.GetBool(pb.Close) {
		return nil, fmt.Errorf("already closed")
	}

	// Create the post body
	cls := true
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{Close: &cls})

	// Create a new comment using the post body
	_, ref, err := args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type:  plumbing.IssueBranchPrefix,
		ID:    args.Reference,
		Body:  postBody,
		Force: args.Force,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create or add new close comment")
	}

	return &IssueCloseResult{Reference: ref}, nil
}
