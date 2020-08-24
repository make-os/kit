package mergecmd

import (
	"fmt"

	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/types"
	"github.com/pkg/errors"
)

// MergeReqFetchArgs contains parameters for MergeReqFetchCmd
type MergeReqFetchArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// ForceFetch forcefully fetches the target
	ForceFetch bool

	// Remote dictates which git remote to fetch target from
	Remote string

	// Base indicates that the base branch should be checked out instead of target
	Base bool

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader
}

// MergeReqFetchCmd fetches a merge request target or base branch
func MergeReqFetchCmd(r types.LocalRepo, args *MergeReqFetchArgs) error {

	selectedBranch, selectedHash, err := getMergeRequestTarget(r, args.Reference, args.ReadPostBody, args.Base)
	if err != nil {
		return err
	}

	targetLbl := "target"
	if args.Base {
		targetLbl = "base"
	}

	if args.Remote == "" {
		args.Remote = "origin"
	}

	// Ensure we have a branch and branch hash to checkout
	if selectedBranch == "" {
		return fmt.Errorf("%s branch was not set in merge request", targetLbl)
	} else if selectedHash == "" {
		return fmt.Errorf("%s branch hash was not set in merge request", targetLbl)
	}

	// Fetch the target branch
	fetchArgs := types.RefFetchArgs{Remote: args.Remote, RemoteRef: selectedBranch,
		LocalRef: selectedBranch, Force: args.ForceFetch, Verbose: true}
	if err := r.RefFetch(fetchArgs); err != nil {
		return errors.Wrapf(err, "failed to fetch %s branch", targetLbl)
	}

	return nil
}
