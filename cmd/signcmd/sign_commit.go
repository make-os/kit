package signcmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	pl "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/api"
	errors2 "github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
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

	// MergeID indicates an optional merge proposal ID to attach to the transaction
	MergeID string

	// Head specifies a reference to use in the transaction info instead of the signed branch reference
	Head string

	// PushKeyID is the signers push key ID
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient types2.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// CreateApplyPushTokenToRemote is a function for creating and applying push tokens on a git remote
	CreateApplyPushTokenToRemote server.MakeAndApplyPushTokenToRemoteFunc

	Stdout io.Writer
	Stderr io.Writer
}

var ErrMissingPushKeyID = fmt.Errorf("push key ID is required")

type SignCommitFunc func(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error

// SignCommitCmd creates and signs a push token for a commit.
//  - cfg: App config object
//  - repo: The target repository at the working directory
//  - args: Arguments
func SignCommitCmd(cfg *config.AppConfig, repo types.LocalRepo, args *SignCommitArgs) error {
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

	// Updated the pushkey's passphrase to the passphrase that was used to unlock the key.
	// This is required when the passphrase was gotten via an interactive prompt.
	args.PushKeyPass = objx.New(key.GetMeta().Map()).Get("passphrase").Str(args.PushKeyPass)

	// If MergeID is set, validate it.
	if args.MergeID != "" {
		err = validation.CheckMergeProposalID(args.MergeID, -1)
		if err != nil {
			return fmt.Errorf(err.(*util.BadFieldError).Msg)
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
func populateSignCommitArgsFromRepoConfig(repo types.LocalRepo, args *SignCommitArgs) {
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
