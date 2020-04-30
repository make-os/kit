package signcmd

import (
	"fmt"

	"github.com/spf13/pflag"
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

type SignTagArgs struct {
	// Message is a custom tag message
	Message string

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

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
func SignTagCmd(cfg *config.AppConfig, gitArgs []string, targetRepo core.BareRepo, args SignTagArgs) error {

	gitFlags := pflag.NewFlagSet("git-tag", pflag.ExitOnError)
	gitFlags.ParseErrorsWhitelist.UnknownFlags = true
	gitFlags.StringP("message", "m", "", "user message")
	gitFlags.StringP("local-user", "u", "", "user signing key")
	gitFlags.Parse(gitArgs)

	// If --local-user (-u) flag is provided in the git args, use the value as the push key ID
	if gitFlags.Lookup("local-user") != nil {
		args.PushKeyID, _ = gitFlags.GetString("local-user")
	}

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

	// If --message (-m) flag is provided, use the value as the message
	if gitFlags.Lookup("message") != nil {
		args.Message, _ = gitFlags.GetString("message")
	}

	// Remove -m, --message, -u, -local-user flag from the git args
	gitArgs = util.RemoveFlag(gitArgs, []string{"m", "message", "u", "local-user"})

	// Get the next nonce, if not set
	if util.IsZeroString(args.Nonce) {
		args.Nonce, err = api.GetNextNonceOfPushKeyOwner(args.PushKeyID, args.RPCClient, args.RemoteClients)
		if err != nil {
			return err
		}
	}

	// Gather any transaction params options
	options := []string{
		fmt.Sprintf("reference=%s", plumbing.NewTagReferenceName(gitFlags.Arg(0)).String()),
	}

	// Construct the txDetail and append to the current message
	txDetail, err := types.MakeAndValidateTxDetail(args.Fee, args.Nonce, args.PushKeyID, nil, options...)
	if err != nil {
		return err
	}

	// Create & set request token to remote URLs in config
	if _, err = server.UpdateRemoteURLsWithPushToken(targetRepo, args.Remote, txDetail,
		key, args.ResetTokens); err != nil {
		return err
	}

	// Create the tag
	if err = targetRepo.CreateTagWithMsg(gitArgs, args.Message, args.PushKeyID); err != nil {
		return err
	}

	return nil
}
