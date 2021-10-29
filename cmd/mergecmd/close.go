package mergecmd

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/pkg/errors"
)

// MergeReqCloseArgs contains parameters for MergeReqCloseCmd
type MergeReqCloseArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	// Force indicates that uncommitted changes should be ignored
	Force bool
}

type MergeReqCloseResult struct {
	Reference string
}

// MergeReqCloseCmdFunc describes MergeReqCloseCmd function signature
type MergeReqCloseCmdFunc func(r plumbing.LocalRepo, args *MergeReqCloseArgs) (*MergeReqCloseResult, error)

// MergeReqCloseCmd adds a close directive
func MergeReqCloseCmd(r plumbing.LocalRepo, args *MergeReqCloseArgs) (*MergeReqCloseResult, error) {

	// Ensure the merge request reference exist
	recentCommentHash, err := r.RefGet(args.Reference)
	if err != nil {
		if err == plumbing.ErrRefNotFound {
			return nil, fmt.Errorf("merge request not found")
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
		Type:  plumbing.MergeRequestBranchPrefix,
		ID:    args.Reference,
		Body:  postBody,
		Force: args.Force,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create or add new close comment")
	}

	return &MergeReqCloseResult{Reference: ref}, nil
}
