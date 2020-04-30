package signcmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/cmd"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type SignNoteArgs struct {
	// Name is the name of the target note
	Name string

	// Fee is the transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce string

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
}

// SignNoteCmd creates adds transaction information to a note and signs it.
func SignNoteCmd(cfg *config.AppConfig, targetRepo core.BareRepo, args SignNoteArgs) error {

	// Get the signing key id from the git config if not passed as an argument
	if args.PushKeyID == "" {
		args.PushKeyID = targetRepo.GetConfig("user.signingKey")
		if args.PushKeyID == "" {
			return ErrMissingPushKeyID
		}
	}

	// Get and unlock the pusher key
	key, err := cmd.UnlockPushKey(cfg, args.PushKeyID, args.PushKeyPass, targetRepo)
	if err != nil {
		return err
	}

	// Expand note name to full reference name if name is short
	if !strings.HasPrefix("refs/notes", args.Name) {
		args.Name = "refs/notes/" + args.Name
	}

	// Get the HEAD hash of the note and add it as a option
	noteRef, err := targetRepo.Reference(plumbing.ReferenceName(args.Name), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}
	options := []string{
		fmt.Sprintf("reference=%s", noteRef.Name()),
		fmt.Sprintf("head=%s", noteRef.Hash().String()),
	}

	// Get the next nonce, if not set
	if util.IsZeroString(args.Nonce) {
		args.Nonce, err = api.GetNextNonceOfPushKeyOwner(args.PushKeyID, args.RPCClient, args.RemoteClients)
		if err != nil {
			return err
		}
	}

	// Construct the txDetail
	txDetail, err := types.MakeAndValidateTxDetail(args.Fee, args.Nonce, args.PushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = server.UpdateRemoteURLsWithPushToken(targetRepo, args.Remote, txDetail, key, args.ResetTokens); err != nil {
		return err
	}

	return nil
}
