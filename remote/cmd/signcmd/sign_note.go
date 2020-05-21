package signcmd

import (
	"strconv"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/cmd"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type SignNoteArgs struct {
	// Name is the name of the target note
	Name string

	// Fee is the transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

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
	key, err := args.PushKeyUnlocker(cfg, args.PushKeyID, args.PushKeyPass, targetRepo)
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
