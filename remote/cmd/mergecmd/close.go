package mergecmd

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
)

// MergeReqCloseArgs contains parameters for MergeReqCloseCmd
type MergeReqCloseArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// PostCommentCreator is the post commit creating function
	PostCommentCreator plumbing.PostCommitCreator

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader
}

// MergeReqCloseCmd adds a close directive
func MergeReqCloseCmd(r types.LocalRepo, args *MergeReqCloseArgs) error {

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
	} else if pb.Close != nil && *pb.Close {
		return fmt.Errorf("already closed")
	}

	// Create the post body
	cls := true
	postBody := plumbing.PostBodyToString(&plumbing.PostBody{Close: &cls})

	// Create a new comment using the post body
	_, _, err = args.PostCommentCreator(r, &plumbing.CreatePostCommitArgs{
		Type: plumbing.MergeRequestBranchPrefix,
		ID:   args.Reference,
		Body: postBody,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create or add new close comment")
	}

	return nil
}
