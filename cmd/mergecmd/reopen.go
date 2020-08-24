package mergecmd

import (
	"fmt"

	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/types"
	"github.com/pkg/errors"
)

// MergeReqReopenArgs contains parameters for MergeReqReopenCmd
type MergeReqReopenArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	// Force indicates that uncommitted changes should be ignored
	Force bool
}

// MergeReqReopenCmd adds a negative close directive to a merge request
func MergeReqReopenCmd(r types.LocalRepo, args *MergeReqReopenArgs) error {

	// Ensure the merge request reference exist
	recentCommentHash, err := r.RefGet(args.Reference)
	if err != nil {
		if err == plumbing.ErrRefNotFound {
			return fmt.Errorf("merge request not found")
		}
		return err
	}

	pb, _, err := args.ReadPostBody(r, recentCommentHash)
	if err != nil {
		return errors.Wrap(err, "failed to read recent comment")
	} else if pb.Close != nil && !(*pb.Close) {
		return fmt.Errorf("already open")
	}

	// Create the post body
	cls := false
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{Close: &cls})

	// Create a new comment using the post body
	_, _, err = args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type:  plumbing.MergeRequestBranchPrefix,
		ID:    args.Reference,
		Body:  postBody,
		Force: args.Force,
	})
	if err != nil {
		return errors.Wrap(err, "failed to add comment")
	}

	return nil
}
