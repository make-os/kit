package mergecmd

import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
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
	ConfirmInput util.ConfirmInputReader

	StdOut io.Writer
}

// MergeReqCheckoutCmd checkouts a merge request target
func MergeReqCheckoutCmd(r types.LocalRepo, args *MergeReqCheckoutArgs) error {

	hashes, err := r.GetRefCommits(args.Reference, true)
	if err != nil {
		if err == plumbing.ErrRefNotFound {
			return fmt.Errorf("merge request not found")
		}
		return err
	}

	// Get the target branch name and target hash.
	// If args.Base is true, find only the base and base hash.
	// Exit the search once we find what we are looking for.
	var base, baseHash, target, targetHash, sBranch, sHash string
	for _, hash := range hashes {
		pb, _, err := args.ReadPostBody(r, hash)
		if err != nil {
			return errors.Wrapf(err, "failed to read commit (%s)", hash[:7])
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

		if !args.Base {
			sBranch = util.NonZeroOrDefString(target, sBranch)
			sHash = util.NonZeroOrDefString(targetHash, sHash)
		} else {
			sBranch = util.NonZeroOrDefString(base, sBranch)
			sHash = util.NonZeroOrDefString(baseHash, sHash)
		}
	}

	targetLbl := "target"
	if args.Base {
		targetLbl = "base"
	}

	if args.Remote == "" {
		args.Remote = "origin"
	}

	// Ensure we have a branch and branch hash to checkout
	if sBranch == "" {
		return fmt.Errorf("%s branch was not set in merge request", targetLbl)
	} else if sHash == "" {
		return fmt.Errorf("%s branch hash was not set in merge request", targetLbl)
	}

	// Fetch the target branch
	fetchArgs := types.RefFetchArgs{Remote: args.Remote, RemoteRef: sBranch, LocalRef: sBranch, Force: args.ForceFetch}
	if err := r.RefFetch(fetchArgs); err != nil {
		return errors.Wrap(err, "failed to fetch target")
	}

	hash, err := r.RefGet(sBranch)
	if err != nil {
		return err
	}

	// If the branch hash from the merge request matches the
	// current hash of the same branch locally or "Yes"
	// is true, check it out
	if sHash == hash || args.YesCheckoutDiffTarget {
		goto checkout
	}

	// At this point, the local branch and the merge request hash
	// are not in sync. We need to inform the user and ask them to
	// confirm whether they want us to check it out
	fmt.Fprintf(args.StdOut, color.YellowString("The %s merge request branch tip (%s) differs "+
		"from the local tip. \n"), targetLbl, sBranch, sBranch)
	if !args.ConfirmInput("\u001B[1;32m? \u001B[1;37mDo you wish to continue checkout? \u001B[0m", false) {
		fmt.Fprint(args.StdOut, "\n")
		return fmt.Errorf("aborted")
	}

checkout:
	if err = r.Checkout(sBranch, false, args.ForceCheckout); err != nil {
		return errors.Wrapf(err, "failed to checkout %s branch (%s)", targetLbl, sBranch)
	}

	return nil
}
