package mergecmd

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
)

// MergeReqReopenArgs contains parameters for MergeReqReopenCmd
type MergeReqReopenArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader
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
		return fmt.Errorf("already opened")
	}

	// Create the post body
	cls := false
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{Close: &cls})

	// Create a new comment using the post body
	_, _, err = args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type: plumbing.MergeRequestBranchPrefix,
		ID:   args.Reference,
		Body: postBody,
	})
	if err != nil {
		return errors.Wrap(err, "failed to add comment")
	}

	return nil
}
