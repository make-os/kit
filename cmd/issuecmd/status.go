package issuecmd

import (
	"fmt"
	"io"

	"github.com/make-os/kit/remote/plumbing"
	"github.com/pkg/errors"
)

// IssueStatusArgs contains parameters for IssueStatusCmd
type IssueStatusArgs struct {

	// Reference is the full reference path to the issue
	Reference string

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	StdOut io.Writer
}

// IssueStatusCmd prints the status of an issue
func IssueStatusCmd(r plumbing.LocalRepo, args *IssueStatusArgs) error {

	// Ensure the issue reference exist
	recentCommentHash, err := r.RefGet(args.Reference)
	if err != nil {
		if err == plumbing.ErrRefNotFound {
			return fmt.Errorf("issue not found")
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

	fmt.Fprintf(args.StdOut, "open\n")

	return nil
}
