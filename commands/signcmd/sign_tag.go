package signcmd

import (
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/utils"
	"gitlab.com/makeos/mosdef/commands/common"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/server"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type SignTagArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// Message is a custom tag message
	Message string

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

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
func SignTagCmd(cfg *config.AppConfig, gitArgs []string, targetRepo types.LocalRepo, args *SignTagArgs) error {

	gitFlags := pflag.NewFlagSet("git-tag", pflag.ExitOnError)
	gitFlags.ParseErrorsWhitelist.UnknownFlags = true
	gitFlags.StringP("message", "m", "", "user message")
	gitFlags.StringP("local-user", "u", "", "user signing key")
	gitFlags.Parse(gitArgs)

	// If --local-user (-u) flag is provided in the git args, use the value as the push key ID
	if gitFlags.Lookup("local-user").Changed {
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

	// If --message (-m) flag is provided, use the value as the message
	if gitFlags.Lookup("message").Changed {
		args.Message, _ = gitFlags.GetString("message")
	}

	// Remove -m, --message, -u, -local-user flag from the git args
	gitArgs = util.RemoveFlag(gitArgs, []string{"m", "message", "u", "local-user"})

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
		Reference: plumbing.NewTagReferenceName(gitFlags.Arg(0)).String(),
	}

	// Create & set request token to remote URLs in config
	if _, err = args.RemoteURLTokenUpdater(targetRepo, args.Remote, txDetail,
		key, args.ResetTokens); err != nil {
		return err
	}

	// If the APPNAME_REPONAME_PASS var is unset, set it to the user-defined push key pass.
	// This is required to allow git-sign learn the passphrase for unlocking the push key.
	// If we met it unset, set a deferred function to unset the var once done.
	passVar := common.MakeRepoScopedPassEnvVar(config.AppName, targetRepo.GetName())
	if len(os.Getenv(passVar)) == 0 {
		os.Setenv(passVar, args.PushKeyPass)
		defer func() { os.Setenv(passVar, "") }()
	}

	// Create the tag
	if err = targetRepo.CreateTagWithMsg(gitArgs, args.Message, args.PushKeyID); err != nil {
		return err
	}

	return nil
}
