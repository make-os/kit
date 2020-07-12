package signcmd

import (
	"strconv"

	"github.com/pkg/errors"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/utils"
	"gitlab.com/makeos/mosdef/commands/common"
	"gitlab.com/makeos/mosdef/config"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
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
	PushKeyID string

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

// SignNoteCmd creates adds transaction information to a note and signs it.
func SignNoteCmd(cfg *config.AppConfig, targetRepo types.LocalRepo, args *SignNoteArgs) error {

	// Get the signing key id from the git config if not passed as an argument
	if args.PushKeyID == "" {
		args.PushKeyID = targetRepo.GetConfig("user.signingKey")
		if args.PushKeyID == "" {
			return ErrMissingPushKeyID
		}
	}

	// Get and unlock the pusher key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: args.PushKeyID,
		Passphrase:   args.PushKeyPass,
		AskPass:      true,
		TargetRepo:   targetRepo,
		Prompt:       "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors.Wrap(err, "failed to unlock push key")
	}

	// Expand note name to full reference name if name is short
	if !plumbing2.IsReference(args.Name) {
		args.Name = "refs/notes/" + args.Name
	}

	// Get the HEAD hash of the note and add it as a option
	noteRef, err := targetRepo.Reference(plumbing.ReferenceName(args.Name), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(args.PushKeyID, args.RPCClient, args.RemoteClients)
		if err != nil {
			return err
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Make the transaction parameter object
	txDetail := &types.TxDetail{
		Fee:       util.String(args.Fee),
		Value:     util.String(args.Value),
		Nonce:     args.Nonce,
		PushKeyID: args.PushKeyID,
		Reference: noteRef.Name().String(),
		Head:      noteRef.Hash().String(),
	}

	// Create & set request token to remote URLs in config
	if _, err = args.RemoteURLTokenUpdater(targetRepo, args.Remote, txDetail,
		key, args.ResetTokens); err != nil {
		return err
	}

	return nil
}
