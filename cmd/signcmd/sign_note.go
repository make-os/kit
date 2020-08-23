package signcmd

import (
	"io"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/config"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/server"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/util"
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

	// SetRemotePushTokensOptionOnly indicates that only remote.*.tokens should hold the push token
	SetRemotePushTokensOptionOnly bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce utils.NextNonceGetter

	SetRemotePushToken server.RemotePushTokenSetter

	Stdout io.Writer
	Stderr io.Writer
}

type SignNoteFunc func(cfg *config.AppConfig, repo types.LocalRepo, args *SignNoteArgs) error

// SignNoteCmd creates adds transaction information to a note and signs it.
func SignNoteCmd(cfg *config.AppConfig, repo types.LocalRepo, args *SignNoteArgs) error {

	populateSignNoteArgsFromRepoConfig(repo, args)

	// Get the signing key id from the git config if not passed as an argument
	if args.SigningKey == "" {
		args.SigningKey = repo.GetConfig("user.signingKey")
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

	// Updated the push key passphrase to the actual passphrase used to unlock the key.
	// This is required when the passphrase was gotten via an interactive prompt.
	args.PushKeyPass = objx.New(key.GetMeta()).Get("passphrase").Str(args.PushKeyPass)

	// Expand note name to full reference name if name is short
	if !plumbing2.IsReference(args.Name) {
		args.Name = "refs/notes/" + args.Name
	}

	// Get the HEAD hash of the note and add it as a option
	noteRef, err := repo.Reference(plumbing.ReferenceName(args.Name), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient, args.RemoteClients)
		if err != nil {
			return errors.Wrapf(err, "failed to get next nonce")
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Create & set push request token to remote URLs in config
	if _, err = args.SetRemotePushToken(repo, &server.SetRemotePushTokenArgs{
		TargetRemote:                  args.Remote,
		PushKey:                       key,
		SetRemotePushTokensOptionOnly: args.SetRemotePushTokensOptionOnly,
		ResetTokens:                   args.ResetTokens,
		Stderr:                        args.Stderr,
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
		args.SigningKey = repo.GetConfig("user.signingKey")
	}
	if args.PushKeyPass == "" {
		args.PushKeyPass = repo.GetConfig("user.passphrase")
	}
	if args.Fee == "" {
		args.Fee = repo.GetConfig("user.fee")
	}
	if args.Nonce == 0 {
		args.Nonce = cast.ToUint64(repo.GetConfig("user.nonce"))
	}
	if args.Value == "" {
		args.Value = repo.GetConfig("user.value")
	}
	if args.SetRemotePushTokensOptionOnly == false {
		args.SetRemotePushTokensOptionOnly = cast.ToBool(repo.GetConfig("sign.noUsername"))
	}
}
