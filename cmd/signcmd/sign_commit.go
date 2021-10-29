package signcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/cmd/common"
	types3 "github.com/make-os/kit/cmd/signcmd/types"
	"github.com/make-os/kit/config"
	pl "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	errors2 "github.com/pkg/errors"
	"github.com/spf13/cast"
)

var ErrMissingPushKeyID = fmt.Errorf("push key ID is required")

// SignCommitCmd creates and signs a push token for a commit.
//  - cfg: App config object
//  - repo: The target repository at the working directory
//  - args: Arguments
func SignCommitCmd(cfg *config.AppConfig, repo pl.LocalRepo, args *types3.SignCommitArgs) error {
	populateSignCommitArgsFromRepoConfig(repo, args)

	// Set merge ID from env if unset
	if args.MergeID == "" {
		args.MergeID = strings.ToUpper(os.Getenv(fmt.Sprintf("%s_MR_ID", cfg.GetAppName())))
	}

	// Signing key is required
	if args.SigningKey == "" {
		return ErrMissingPushKeyID
	}

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.PushKeyPass,
		NoPrompt:   args.NoPrompt,
		TargetRepo: repo,
		Stdout:     args.Stdout,
		Prompt:     "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors2.Wrap(err, "failed to unlock the signing key")
	}

	// Get push key from key (args.SigningKey may not be push key address)
	pushKeyID := key.GetPushKeyAddress()

	// If MergeID is set, validate it.
	if args.MergeID != "" {
		err = validation.CheckMergeProposalID(args.MergeID, -1)
		if err != nil {
			return fmt.Errorf(err.(*errors.BadFieldError).Msg)
		}
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient)
		if err != nil {
			return errors2.Wrapf(err, "failed to get next nonce")
		}
		args.Nonce = cast.ToUint64(nonce)
	}

	// Get the current HEAD reference.
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD")
	}
	if args.Head != "" {
		head = args.Head
	}
	if !pl.IsReference(head) {
		head = plumbing.NewBranchReferenceName(args.Head).String()
	}

	// Get the HEAD reference object
	headRef, err := repo.Reference(plumbing.ReferenceName(head), false)
	if err != nil {
		return errors2.Wrapf(err, "failed to find reference: %s", head)
	}

	if err = args.CreateApplyPushTokenToRemote(repo, &server.MakeAndApplyPushTokenToRemoteArgs{
		TargetRemote: args.Remote,
		PushKey:      key,
		Stderr:       args.Stderr,
		ResetTokens:  args.ResetTokens,
		TxDetail: &types.TxDetail{
			Fee:             util.String(args.Fee),
			Value:           util.String(args.Value),
			Nonce:           args.Nonce,
			PushKeyID:       pushKeyID,
			MergeProposalID: args.MergeID,
			Reference:       head,
			Head:            headRef.Hash().String(),
		},
	}); err != nil {
		return err
	}

	return nil
}

// populateSignCommitArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignCommitArgsFromRepoConfig(repo pl.LocalRepo, args *types3.SignCommitArgs) {
	if args.SigningKey == "" {
		args.SigningKey = repo.GetGitConfigOption("user.signingKey")
	}
	if args.PushKeyPass == "" {
		args.PushKeyPass = repo.GetGitConfigOption("user.passphrase")
	}
	if util.IsZeroString(args.Fee) {
		args.Fee = repo.GetGitConfigOption("user.fee")
	}
	if args.Nonce == 0 {
		args.Nonce = cast.ToUint64(repo.GetGitConfigOption("user.nonce"))
	}
	if util.IsZeroString(args.Value) {
		args.Value = repo.GetGitConfigOption("user.value")
	}
	if args.MergeID == "" {
		args.MergeID = repo.GetGitConfigOption("sign.mergeID")
	}
}
