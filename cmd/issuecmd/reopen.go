package issuecmd

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/pkg/errors"
)

// IssueReopenArgs contains parameters for IssueReopenCmd
type IssueReopenArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	// Force indicates that uncommitted changes should be ignored
	Force bool
}

type IssueReopenResult struct {
	Reference string
}

// IssueReopenCmdFunc describes IssueReopenCmd function signature
type IssueReopenCmdFunc func(r types.LocalRepo, args *IssueReopenArgs) (*IssueReopenResult, error)

// IssueReopenCmd adds a negative close directive to an issue
func IssueReopenCmd(r types.LocalRepo, args *IssueReopenArgs) (*IssueReopenResult, error) {

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
	} else if !pointer.GetBool(pb.Close) {
		return nil, fmt.Errorf("already open")
	}

	// Create the post body
	cls := false
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{Close: &cls})

	// Create a new comment using the post body
	_, ref, err := args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type:  plumbing.IssueBranchPrefix,
		ID:    args.Reference,
		Body:  postBody,
		Force: args.Force,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to add comment")
	}

	return &IssueReopenResult{Reference: ref}, nil
}
