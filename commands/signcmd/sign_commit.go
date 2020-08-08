package signcmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	errors2 "github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/config"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/server"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/remote/validation"
	"github.com/themakeos/lobe/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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

	// SetRemotePushTokensOptionOnly indicates that only remote.*.tokens should hold the push token
	SetRemotePushTokensOptionOnly bool

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce utils.NextNonceGetter

	// SetRemotePushToken is a function for setting push tokens on a git remote config
	SetRemotePushToken server.RemotePushTokenSetter
}

var ErrMissingPushKeyID = fmt.Errorf("push key ID is required")

type SignCommitFunc func(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error

// SignCommitCmd adds transaction information to a new or recent commit and signs it.
// cfg: App config object
// repo: The target repository at the working directory
// args: Arguments
func SignCommitCmd(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error {

	populateSignCommitArgsFromRepoConfig(repo, args)

	// Set merge ID from env if unset
	if args.MergeID == "" {
		args.MergeID = strings.ToUpper(os.Getenv(fmt.Sprintf("%s_MR_ID", cfg.GetExecName())))
	}

	// Signing key is required
	if args.SigningKey == "" {
		return ErrMissingPushKeyID
	}

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: args.SigningKey,
		Passphrase:   args.PushKeyPass,
		AskPass:      true,
		TargetRepo:   repo,
		Prompt:       "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors2.Wrap(err, "failed to unlock the signing key")
	}

	// Get push key from key (args.SigningKey may not be push key address)
	pushKeyID := key.GetPushKeyAddress()

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
			return errors2.Wrapf(err, "get-next-nonce error")
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
		if err := repo.Checkout(plumbing.ReferenceName(repoHead).Short(), false,
			args.ForceCheckout); err != nil {
			return fmt.Errorf("failed to checkout branch (%s): %s", repoHead, err)
		}
		defer func() {
			_ = repo.Checkout(plumbing.ReferenceName(curHead).Short(), false, false)
		}()
	}

	// Use active branch as the tx reference only if args.HEAD was not explicitly provided
	var reference = repoHead
	if args.Head != "" {
		reference = args.Head
		if !plumbing2.IsReference(reference) {
			reference = plumbing.NewBranchReferenceName(args.Head).String()
		}
	}

	// If the APPNAME_REPONAME_PASS var is unset, set it to the user-defined push key pass.
	// This is required to allow git-sign learn the passphrase to unlock the push key.
	passVar := common.MakeRepoScopedEnvVar(cfg.GetExecName(), repo.GetName(), "PASS")
	if len(os.Getenv(passVar)) == 0 {
		os.Setenv(passVar, args.PushKeyPass)
	}

	var commit *object.Commit
	var hash string

	// Create a new commit if recent commit amendment is not desired.
	if !args.AmendCommit {
		if err := repo.CreateSignedEmptyCommit(args.Message, args.SigningKey); err != nil {
			return err
		}
		goto create_token
	}

	// Otherwise, amend the recent commit.
	// Get recent commit hash of the current branch.
	hash, err = repo.GetRecentCommitHash()
	if err != nil {
		if err == plumbing2.ErrNoCommits {
			return errors.New("no commits have been created yet")
		}
		return err
	}

	// Use previous commit message as default
	commit, err = repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return err
	} else if args.Message == "" {
		args.Message = commit.Message
	}

	// Update the recent commit message.
	if err = repo.UpdateRecentCommitMsg(args.Message, args.SigningKey); err != nil {
		return err
	}

	// Create & set push request token on the remote in config.
	// Also get the post-sign hash of the current branch.
create_token:
	hash, _ = repo.GetRecentCommitHash()
	if _, err = args.SetRemotePushToken(cfg, repo, &server.SetRemotePushTokenArgs{
		TargetRemote:                  args.Remote,
		PushKey:                       key,
		SetRemotePushTokensOptionOnly: args.SetRemotePushTokensOptionOnly,
		ResetTokens:                   args.ResetTokens,
		TxDetail: &types.TxDetail{
			Fee:             util.String(args.Fee),
			Value:           util.String(args.Value),
			Nonce:           args.Nonce,
			PushKeyID:       pushKeyID,
			MergeProposalID: args.MergeID,
			Reference:       reference,
			Head:            hash,
		},
	}); err != nil {
		return err
	}

	return nil
}

// populateSignCommitArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignCommitArgsFromRepoConfig(repo types.LocalRepo, args *SignCommitArgs) {
	if args.SigningKey == "" {
		args.SigningKey = repo.GetConfig("user.signingKey")
	}
	if args.PushKeyPass == "" {
		args.PushKeyPass = repo.GetConfig("user.passphrase")
	}
	if util.IsZeroString(args.Fee) {
		args.Fee = repo.GetConfig("user.fee")
	}
	if args.Nonce == 0 {
		args.Nonce = cast.ToUint64(repo.GetConfig("user.nonce"))
	}
	if util.IsZeroString(args.Value) {
		args.Value = repo.GetConfig("user.value")
	}
	if args.AmendCommit == false {
		args.AmendCommit = cast.ToBool(repo.GetConfig("commit.amend"))
	}
	if args.SetRemotePushTokensOptionOnly == false {
		args.SetRemotePushTokensOptionOnly = cast.ToBool(repo.GetConfig("sign.noUsername"))
	}
	if args.MergeID == "" {
		args.MergeID = repo.GetConfig("sign.mergeID")
	}
}
