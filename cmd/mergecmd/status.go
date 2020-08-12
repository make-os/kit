package mergecmd

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
)

// MergeReqStatusArgs contains parameters for MergeReqStatusCmd
type MergeReqStatusArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	StdOut io.Writer
}

// MergeReqStatusCmd prints the status of the merge request
func MergeReqStatusCmd(r types.LocalRepo, args *MergeReqStatusArgs) error {

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
	}

	if pb.Close != nil && *pb.Close {
		fmt.Fprintf(args.StdOut, "closed\n")
		return nil
	}

	fmt.Fprintf(args.StdOut, "opened\n")

	return nil
}
