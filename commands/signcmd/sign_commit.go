package signcmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	errors2 "github.com/pkg/errors"
	"github.com/stretchr/objx"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/server"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/remote/validation"
	"github.com/themakeos/lobe/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type SignCommitArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// Message is a custom commit message
	Message string

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
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce utils.NextNonceGetter

	// RemoteURLTokenUpdater is a function for setting push tokens to git remote URLs
	RemoteURLTokenUpdater server.RemoteURLsPushTokenUpdater
}

var ErrMissingPushKeyID = fmt.Errorf("push key ID is required")

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
// cfg: App config object
// targetRepo: The target repository at the working directory
func SignCommitCmd(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error {

	// If signing key was not provided, use signing key id from the git config
	if args.SigningKey == "" {
		args.SigningKey = repo.GetConfig("user.signingKey")
		if args.SigningKey == "" {
			return ErrMissingPushKeyID
		}
	}

	// Get and unlock the signing key
	pushKeyID := args.SigningKey
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: pushKeyID,
		Passphrase:   args.PushKeyPass,
		AskPass:      true,
		TargetRepo:   repo,
		Prompt:       "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors2.Wrap(err, "failed to unlock the signing key")
	} else if crypto.IsValidUserAddr(key.GetUserAddress()) == nil {
		// If the key's address is not a push key address, then we need to convert to a push key address
		// as that is what the remote node will accept. We do this because we assume the user wants to
		// use a user key to sign.
		pushKeyID = key.GetKey().PushAddr().String()
	}

	// Updated the push key passphrase to the actual passphrase used to unlock the key.
	// This is required when the passphrase was gotten via an interactive prompt.
	args.PushKeyPass = objx.New(key.GetMeta()).Get("passphrase").Str(args.PushKeyPass)

	// if MergeID is set, validate it.
	if args.MergeID != "" {
		err = validation.CheckMergeProposalID(args.MergeID, -1)
		if err != nil {
			return fmt.Errorf(err.(*util.BadFieldError).Msg)
		}
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient, args.RemoteClients)
		if err != nil {
			return err
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Get the current active branch.
	// When branch is explicitly provided, use it as the active branch
	var curHead string
	repoHead, err := repo.Head()
	curHead = repoHead
	if err != nil {
		return fmt.Errorf("failed to get HEAD")
	} else if args.Branch != "" {
		repoHead = args.Branch
		if !plumbing2.IsReference(repoHead) {
			repoHead = plumbing.NewBranchReferenceName(repoHead).String()
		}
	}

	// If an explicit branch was provided via flag, check it out.
	// Then set a deferred function to revert back the the original branch.
	if repoHead != curHead {
		if err := repo.Checkout(plumbing.ReferenceName(repoHead).Short(),
			false, args.ForceCheckout); err != nil {
			return fmt.Errorf("failed to checkout branch (%s): %s", repoHead, err)
		}
		defer func() {
			_ = repo.Checkout(plumbing.ReferenceName(curHead).Short(), false, false)
		}()
	}

	// Use active branch as the tx reference only if HEAD arg. was not explicitly provided
	var reference = repoHead
	if args.Head != "" {
		reference = args.Head
		if !plumbing2.IsReference(reference) {
			reference = plumbing.NewBranchReferenceName(args.Head).String()
		}
	}

	// Make the transaction parameter object
	txDetail := &types.TxDetail{
		Fee:             util.String(args.Fee),
		Value:           util.String(args.Value),
		Nonce:           args.Nonce,
		PushKeyID:       pushKeyID,
		MergeProposalID: args.MergeID,
		Reference:       reference,
	}

	// Create & set request token to remote URLs in config
	if _, err = args.RemoteURLTokenUpdater(repo, args.Remote, txDetail, key, args.ResetTokens); err != nil {
		return err
	}

	// If the APPNAME_REPONAME_PASS var is unset, set it to the user-defined push key pass.
	// This is required to allow git-sign learn the passphrase for unlocking the push key.
	// If we met it unset, set a deferred function to unset the var once done.
	passVar := common.MakeRepoScopedPassEnvVar(config.AppName, repo.GetName())
	if len(os.Getenv(passVar)) == 0 {
		_ = os.Setenv(passVar, args.PushKeyPass)
		defer func() { _ = os.Setenv(passVar, "") }()
	}

	// Create a new quiet commit if recent commit amendment is not desired.
	// NOTE: We are using args.SigningKey instead of pushKeyID because pushKeyID
	// may contain push key id derived from a user key specified in args.SigningKey.
	// If this is the case, the derived push key in pushKeyID may not be found by git-sign.
	if !args.AmendCommit {
		if err := repo.CreateSignedEmptyCommit(args.Message, args.SigningKey); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err := repo.GetRecentCommitHash()
	if err != nil {
		if err == plumbing2.ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	// Use previous commit message as default
	commit, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return err
	} else if args.Message == "" {
		args.Message = commit.Message
	}

	// Update the recent commit message.
	// NOTE: We are using args.SigningKey instead of pushKeyID because pushKeyID
	// may contain push key id derived from a user key specified in args.SigningKey.
	// If this is the case, the derived push key in pushKeyID may not be found by git-sign.
	if err = repo.UpdateRecentCommitMsg(args.Message, args.SigningKey); err != nil {
		return err
	}

	return nil
}
