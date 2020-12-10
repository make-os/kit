package signcmd

import (
	"io"
	"strconv"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/remote/types"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/api"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type SignNoteArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// Name is the name of the target note
	Name string

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
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	CreateApplyPushTokenToRemote server.MakeAndApplyPushTokenToRemoteFunc

	Stdout io.Writer
	Stderr io.Writer
}

type SignNoteFunc func(cfg *config.AppConfig, repo types.LocalRepo, args *SignNoteArgs) error

// SignNoteCmd create and signs a push token for a given note
func SignNoteCmd(cfg *config.AppConfig, repo types.LocalRepo, args *SignNoteArgs) error {

	populateSignNoteArgsFromRepoConfig(repo, args)

	// Get the signing key id from the git config if not passed as an argument
	if args.SigningKey == "" {
		args.SigningKey = repo.GetGitConfigOption("user.signingKey")
		if args.SigningKey == "" {
			return ErrMissingPushKeyID
		}
	}

	// Get and unlock the pusher key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.PushKeyPass,
		NoPrompt:   args.NoPrompt,
		TargetRepo: repo,
		Stdout:     args.Stdout,
		Prompt:     "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors.Wrap(err, "failed to unlock push key")
	}

	// Get push key from key (args.SigningKey may not be push key address)
	pushKeyID := key.GetPushKeyAddress()

	// Expand note name to full reference name if name is short
	if !plumbing2.IsReference(args.Name) {
		args.Name = plumbing.NewNoteReferenceName(args.Name).String()
	}

	// Get the note's reference
	noteRef, err := repo.Reference(plumbing.ReferenceName(args.Name), false)
	if err != nil {
		return err
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient)
		if err != nil {
			return errors.Wrapf(err, "failed to get next nonce")
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Create & set push request token to remote URLs in config
	if err = args.CreateApplyPushTokenToRemote(repo, &server.MakeAndApplyPushTokenToRemoteArgs{
		TargetRemote: args.Remote,
		PushKey:      key,
		ResetTokens:  args.ResetTokens,
		Stderr:       args.Stderr,
		TxDetail: &types.TxDetail{
			Fee:       util.String(args.Fee),
			Value:     util.String(args.Value),
			Nonce:     args.Nonce,
			PushKeyID: pushKeyID,
			Reference: noteRef.Name().String(),
			Head:      noteRef.Hash().String(),
		},
	}); err != nil {
		return err
	}

	return nil
}

// populateSignNoteArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignNoteArgsFromRepoConfig(repo types.LocalRepo, args *SignNoteArgs) {
	if args.SigningKey == "" {
		args.SigningKey = repo.GetGitConfigOption("user.signingKey")
	}
	if args.PushKeyPass == "" {
		args.PushKeyPass = repo.GetGitConfigOption("user.passphrase")
	}
	if args.Fee == "" {
		args.Fee = repo.GetGitConfigOption("user.fee")
	}
	if args.Nonce == 0 {
		args.Nonce = cast.ToUint64(repo.GetGitConfigOption("user.nonce"))
	}
	if args.Value == "" {
		args.Value = repo.GetGitConfigOption("user.value")
	}
}
