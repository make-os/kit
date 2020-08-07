package signcmd

import (
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"github.com/stretchr/objx"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/remote/server"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/util"
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
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// Force force git-tag to overwrite existing tag
	Force bool

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// SetRemotePushTokensOptionOnly indicates that only remote.*.tokens should hold the push token
	SetRemotePushTokensOptionOnly bool

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce utils.NextNonceGetter

	// SetRemotePushToken is a function for setting push tokens on a git remote config
	SetRemotePushToken server.RemotePushTokenSetter
}

type SignTagFunc func(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *SignTagArgs) error

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
func SignTagCmd(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *SignTagArgs) error {

	populateSignTagArgsFromRepoConfig(repo, args)

	if args.Force {
		gitArgs = append(gitArgs, "-f")
	}

	gitFlags := pflag.NewFlagSet("git-tag", pflag.ExitOnError)
	gitFlags.ParseErrorsWhitelist.UnknownFlags = true
	gitFlags.StringP("message", "m", "", "user message")
	gitFlags.StringP("local-user", "u", "", "user signing key")
	gitFlags.Parse(gitArgs)

	// If --local-user (-u) flag is provided in the git args, use the value as the push key ID
	if gitFlags.Lookup("local-user").Changed {
		args.SigningKey, _ = gitFlags.GetString("local-user")
	}

	// Get the signing key id from the git config if not passed as an argument
	if args.SigningKey == "" {
		args.SigningKey = repo.GetConfig("user.signingKey")
		if args.SigningKey == "" {
			return ErrMissingPushKeyID
		}
	}

	// Get and unlock the pusher key
	pushKeyID := args.SigningKey
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: pushKeyID,
		Passphrase:   args.PushKeyPass,
		AskPass:      true,
		TargetRepo:   repo,
		Prompt:       "Enter passphrase to unlock the signing key\n",
	})
	if err != nil {
		return errors.Wrap(err, "failed to unlock push key")
	} else if crypto.IsValidUserAddr(key.GetUserAddress()) == nil {
		pushKeyID = key.GetKey().PushAddr().String()
	}

	// Updated the push key passphrase to the actual passphrase used to unlock the key.
	// This is required when the passphrase was gotten via an interactive prompt.
	args.PushKeyPass = objx.New(key.GetMeta()).Get("passphrase").Str(args.PushKeyPass)

	// Get the tag object
	tagRef, err := repo.Tag(gitArgs[0])
	if err != nil && err != plumbing.ErrReferenceNotFound {
		return err
	}
	tag, err := repo.TagObject(tagRef.Hash())
	if err != nil {
		return err
	}

	// Use the existing tag's message as default if the tag already exist.
	if args.Message == "" && tag != nil {
		args.Message = tag.Message
	}

	// If --message (-m) flag is provided, use the value as the message
	if gitFlags.Lookup("message").Changed {
		args.Message, _ = gitFlags.GetString("message")
	}

	// Remove -m, --message, -u, -local-user flag from the git args
	gitArgs = util.RemoveFlag(gitArgs, []string{"m", "message", "u", "local-user"})

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient, args.RemoteClients)
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
		PushKeyID: pushKeyID,
		Reference: plumbing.NewTagReferenceName(gitFlags.Arg(0)).String(),
	}

	// Create & set request token to remote URLs in config
	if _, err = args.SetRemotePushToken(cfg, repo, &server.SetRemotePushTokenArgs{
		TargetRemote:                  args.Remote,
		TxDetail:                      txDetail,
		PushKey:                       key,
		SetRemotePushTokensOptionOnly: args.SetRemotePushTokensOptionOnly,
		ResetTokens:                   args.ResetTokens,
	}); err != nil {
		return err
	}

	// If the APPNAME_REPONAME_PASS var is unset, set it to the user-defined push key pass.
	// This is required to allow git-sign learn the passphrase for unlocking the push key.
	// If we met it unset, set a deferred function to unset the var once done.
	passVar := common.MakeRepoScopedPassEnvVar(cfg.GetExecName(), repo.GetName())
	if len(os.Getenv(passVar)) == 0 {
		os.Setenv(passVar, args.PushKeyPass)
		defer func() { os.Setenv(passVar, "") }()
	}

	// Create the signed tag.
	// NOTE: We are using args.SigningKey instead of pushKeyID because pushKeyID
	// may contain push key id derived from a user key specified in args.SigningKey.
	// If this is the case, the derived push key in pushKeyID may not be found by git-sign.
	if err = repo.CreateTagWithMsg(gitArgs, args.Message, args.SigningKey); err != nil {
		return err
	}

	return nil
}

// populateSignTagArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignTagArgsFromRepoConfig(repo types.LocalRepo, args *SignTagArgs) {
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
