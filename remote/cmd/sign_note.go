package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// SignNoteCmd creates adds transaction information to a note and signs it.
func SignNoteCmd(
	cfg *config.AppConfig,
	targetRepo core.BareRepo,
	txFee,
	nextNonce,
	note,
	pushKeyID,
	pushKeyPass,
	targetRemote string,
	resetTokens bool,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	// Get the signing key id from the git config if not passed as an argument
	if pushKeyID == "" {
		pushKeyID = targetRepo.GetConfig("user.signingKey")
		if pushKeyID == "" {
			return fmt.Errorf("push key ID is required")
		}
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, pushKeyPass, targetRepo)
	if err != nil {
		return err
	}

	// Expand note name to full reference name if name is short
	if !strings.HasPrefix("refs/notes", note) {
		note = "refs/notes/" + note
	}

	// Get the HEAD hash of the note and add it as a option
	noteRef, err := targetRepo.Reference(plumbing.ReferenceName(note), true)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	}
	options := []string{
		fmt.Sprintf("reference=%s", noteRef.Name()),
		fmt.Sprintf("head=%s", noteRef.Hash().String()),
	}

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfPushKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Construct the txDetail
	txDetail, err := types.MakeAndValidateTxDetail(txFee, nextNonce, pushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = server.SetPushTokenToRemotes(targetRepo, targetRemote, txDetail, key, resetTokens); err != nil {
		return err
	}

	return nil
}
