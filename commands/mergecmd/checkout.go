package mergecmd

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
	fmt2 "gitlab.com/makeos/mosdef/util/colorfmt"
	io2 "gitlab.com/makeos/mosdef/util/io"
)

// MergeReqCheckoutArgs contains parameters for MergeReqCheckoutCmd
type MergeReqCheckoutArgs struct {

	// Reference is the full reference path to the merge request
	Reference string

	// ReadPostBody is a function for reading post body in a commit
	ReadPostBody plumbing.PostBodyReader

	// ForceCheckout ignores unsaved local changes and forces checkout
	ForceCheckout bool

	// ForceFetch forcefully fetches the target
	ForceFetch bool

	// Remote dictates which git remote to fetch target from
	Remote string

	// Base indicates that the base branch should be checked out instead of target
	Base bool

	// Yes indicates that all confirm prompts are answered as 'Yes' automatically
	YesCheckoutDiffTarget bool

	// ConfirmInput is a function for requesting user confirmation
	ConfirmInput io2.ConfirmInputReader

	StdOut io.Writer
}

// getMergeRequestTarget takes a merge request reference and extracts merge target.
// r is the local repository.
// reference is the merge request reference
// postBodyReader is a function for reading post body
// useBaseAsTarget will return the base branch and hash as target
func getMergeRequestTarget(
	r types.LocalRepo,
	reference string,
	postBodyReader plumbing.PostBodyReader,
	useBaseAsTarget bool) (target string, targetHash string, err error) {

	hashes, err := r.GetRefCommits(reference, true)
	if err != nil {
		if err == plumbing.ErrRefNotFound {
			return "", "", fmt.Errorf("merge request not found")
		}
		return "", "", err
	}

	// Get the target branch name and target hash.
	// If useBaseAsTarget is true, find only the base and base hash.
	// Exit the search once we find what we are looking for.
	var base, baseHash, selectedBranch, selectedHash string
	for _, hash := range hashes {
		pb, _, err := postBodyReader(r, hash)
		if err != nil {
			return "", "", errors.Wrapf(err, "failed to read commit (%s)", hash[:7])
		}
		if base == "" && pb.BaseBranch != "" {
			base = pb.BaseBranch
		}
		if baseHash == "" && pb.BaseBranchHash != "" {
			baseHash = pb.BaseBranchHash
		}
		if target == "" && pb.TargetBranch != "" {
			target = pb.TargetBranch
		}
		if targetHash == "" && pb.TargetBranchHash != "" {
			targetHash = pb.TargetBranchHash
		}

		if !useBaseAsTarget {
			selectedBranch = util.NonZeroOrDefString(target, selectedBranch)
			selectedHash = util.NonZeroOrDefString(targetHash, selectedHash)
		} else {
			selectedBranch = util.NonZeroOrDefString(base, selectedBranch)
			selectedHash = util.NonZeroOrDefString(baseHash, selectedHash)
		}
	}

	return selectedBranch, selectedHash, nil
}

// MergeReqCheckoutCmd checkouts a merge request target or base branch
func MergeReqCheckoutCmd(r types.LocalRepo, args *MergeReqCheckoutArgs) error {

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

	hash, err := r.RefGet(selectedBranch)
	if err != nil {
		return err
	}

	// If the branch hash from the merge request matches the
	// current hash of the same branch locally or "Yes"
	// is true, check it out
	if selectedHash == hash || args.YesCheckoutDiffTarget {
		goto checkout
	}

	// At this point, the local branch and the merge request hash
	// are not in sync. We need to inform the user and ask them to
	// confirm whether they want us to check it out
	fmt.Fprintf(args.StdOut, fmt2.YellowString("The %s merge request branch tip (%s) differs "+
		"from the local tip. \n"), targetLbl, selectedBranch, selectedBranch)
	if !args.ConfirmInput("\u001B[1;32m? \u001B[1;37mDo you wish to continue checkout? \u001B[0m", false) {
		return fmt.Errorf("aborted")
	}

checkout:
	if err = r.Checkout(selectedBranch, false, args.ForceCheckout); err != nil {
		return errors.Wrapf(err, "failed to checkout %s branch (%s)", targetLbl, selectedBranch)
	}

	return nil
}
