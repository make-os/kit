package signcmd

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/asaskevich/govalidator"
	errors2 "github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/cmd"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type SignCommitArgs struct {
	// Message is a custom commit message
	Message string

	// Fee is the transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// AmendCommit indicates whether to amend the last commit or create an empty commit
	AmendCommit bool

	// MergeID indicates an optional merge proposal ID to attach to the transaction
	MergeID string

	// Head specifies a reference to use in the transaction info instead of the signed branch reference
	Head string

	// Branch specifies a branch to checkout and sign instead of the current branch (HEAD)
	Branch string

	// ForceCheckout forcefully checks out the target branch (clears unsaved work)
	ForceCheckout bool

	// PushKeyID is the signers push key ID
	PushKeyID string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// RpcClient is the RPC client
	RPCClient *client.RPCClient

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.RestClient

	// PushKeyUnlocker is a function for getting and unlocking a push key from keystore
	PushKeyUnlocker cmd.PushKeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// RemoteURLTokenUpdater is a function for setting push tokens to git remote URLs
	RemoteURLTokenUpdater server.RemoteURLsPushTokenUpdater
}

var ErrMissingPushKeyID = fmt.Errorf("push key ID is required")

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
// cfg: App config object
// targetRepo: The target repository at the working directory
func SignCommitCmd(cfg *config.AppConfig, targetRepo core.LocalRepo, args *SignCommitArgs) error {

	// Get the signing key id from the git config if not passed as an argument
	if args.PushKeyID == "" {
		args.PushKeyID = targetRepo.GetConfig("user.signingKey")
		if args.PushKeyID == "" {
			return ErrMissingPushKeyID
		}
	}

	// Get and unlock the pusher key
	key, err := args.PushKeyUnlocker(cfg, args.PushKeyID, args.PushKeyPass, targetRepo)
	if err != nil {
		return errors2.Wrap(err, "unable to unlock push key")
	}

	// Validate merge ID is set.
	// Must be numeric and 8 bytes long
	if args.MergeID != "" {
		if !govalidator.IsNumeric(args.MergeID) {
			return fmt.Errorf("merge id must be numeric")
		} else if len(args.MergeID) > 8 {
			return fmt.Errorf("merge proposal id exceeded 8 bytes limit")
		}
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(args.PushKeyID, args.RPCClient, args.RemoteClients)
		if err != nil {
			return err
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Get the current active branch.
	// When branch is explicitly provided, use it as the active branch
	var activeBranchCpy string
	activeBranch, err := targetRepo.Head()
	activeBranchCpy = activeBranch
	if err != nil {
		return fmt.Errorf("failed to get HEAD")
	} else if args.Branch != "" {
		activeBranch = args.Branch
		if !plumbing2.IsReference(activeBranch) {
			activeBranch = plumbing.NewBranchReferenceName(activeBranch).String()
		}
	}

	// If an explicit branch was provided via flag, check it out.
	// Then set a deferred function to revert back the the original branch.
	if activeBranch != activeBranchCpy {
		if err := targetRepo.Checkout(plumbing.ReferenceName(activeBranch).Short(),
			false, args.ForceCheckout); err != nil {
			return fmt.Errorf("failed to checkout branch (%s): %s", activeBranch, err)
		}
		defer targetRepo.Checkout(plumbing.ReferenceName(activeBranchCpy).Short(), false, false)
	}

	// Use active branch as the tx reference only if
	// head arg. was not explicitly provided
	var reference = activeBranch
	if args.Head != "" {
		reference = args.Head
		if !plumbing2.IsReference(reference) {
			reference = plumbing.NewBranchReferenceName(args.Head).String()
		}
	}

	// Make the transaction parameter object
	txDetail := &core.TxDetail{
		Fee:             util.String(args.Fee),
		Nonce:           args.Nonce,
		PushKeyID:       args.PushKeyID,
		MergeProposalID: args.MergeID,
		Reference:       reference,
	}

	// Create & set request token to remote URLs in config
	if _, err = args.RemoteURLTokenUpdater(targetRepo, args.Remote, txDetail, key, args.ResetTokens); err != nil {
		return err
	}

	// Create a new quiet commit if recent commit amendment is not desired
	if !args.AmendCommit {
		if err := targetRepo.CreateSignedEmptyCommit(args.Message, args.PushKeyID); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err := targetRepo.GetRecentCommitHash()
	if err != nil {
		if err == repo.ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	// Use previous commit message as default
	commit, err := targetRepo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return err
	} else if args.Message == "" {
		args.Message = commit.Message
	}

	// Update the recent commit message
	if err = targetRepo.UpdateRecentCommitMsg(args.Message, args.PushKeyID); err != nil {
		return err
	}

	return nil
}
