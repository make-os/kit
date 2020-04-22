package cmd

import (
	"fmt"

	"github.com/spf13/pflag"
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

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
func SignTagCmd(
	cfg *config.AppConfig,
	gitArgs []string,
	msg string,
	targetRepo core.BareRepo,
	txFee,
	nextNonce,
	pushKeyID,
	pushKeyPass,
	targetRemote string,
	resetTokens bool,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) error {

	gitFlags := pflag.NewFlagSet("git-tag", pflag.ExitOnError)
	gitFlags.ParseErrorsWhitelist.UnknownFlags = true
	gitFlags.StringP("message", "m", "", "user message")
	gitFlags.StringP("local-user", "u", "", "user signing key")
	gitFlags.Parse(gitArgs)

	// If --local-user (-u) flag is provided in the git args, use the value as the push key ID
	if gitFlags.Lookup("local-user") != nil {
		pushKeyID, _ = gitFlags.GetString("local-user")
	}

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

	// If --message (-m) flag is provided, use the value as the message
	if gitFlags.Lookup("message") != nil {
		msg, _ = gitFlags.GetString("message")
	}

	// Remove -m, --message, -u, -local-user flag from the git args
	gitArgs = util.RemoveFlag(gitArgs, []string{"m", "message", "u", "local-user"})

	// Get the next nonce, if not set
	if util.IsZeroString(nextNonce) {
		nextNonce, err = api.DetermineNextNonceOfPushKeyOwner(pushKeyID, rpcClient, remoteClients)
		if err != nil {
			return err
		}
	}

	// Gather any transaction params options
	options := []string{
		fmt.Sprintf("reference=%s", plumbing.NewTagReferenceName(gitFlags.Arg(0)).String()),
	}

	// Construct the txDetail and append to the current message
	txDetail, err := types.MakeAndValidateTxDetail(txFee, nextNonce, pushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = server.SetPushTokenToRemotes(targetRepo, targetRemote, txDetail, key, resetTokens); err != nil {
		return err
	}

	// Create the tag
	if err = targetRepo.CreateTagWithMsg(gitArgs, msg, pushKeyID); err != nil {
		return err
	}

	return nil
}
