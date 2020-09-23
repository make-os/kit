package signcmd

import (
	"io"
	"os"
	"strconv"

	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/remote/server"
	"github.com/make-os/lobe/remote/types"
	types2 "github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/api"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"github.com/stretchr/objx"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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

	// ForceSign forcefully signs a tag when signing is supposed to be skipped
	ForceSign bool

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// SignRefOnly indicates that only the target reference should be signed.
	SignRefOnly bool

	// CreatePushTokenOnly indicates that only the remote token should be created and signed.
	CreatePushTokenOnly bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient types2.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// SetRemotePushToken is a function for setting push tokens on a git remote config
	SetRemotePushToken server.RemotePushTokenSetter

	Stdout io.Writer
	Stderr io.Writer
}

type SignTagFunc func(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *SignTagArgs) error

// SignTagCmd creates an annotated tag, appends transaction information to its
// message and signs it.
func SignTagCmd(cfg *config.AppConfig, gitArgs []string, repo types.LocalRepo, args *SignTagArgs) error {

	populateSignTagArgsFromRepoConfig(repo, args)

	// Set -f flag if Force or ForceSign is true
	if args.Force || args.ForceSign {
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

	// Updated the push key passphrase to the actual passphrase used to unlock the key.
	// This is required when the passphrase was gotten via an interactive prompt.
	args.PushKeyPass = objx.New(key.GetMeta()).Get("passphrase").Str(args.PushKeyPass)

	// Get the tag object if it already exists
	var tag *object.Tag
	tagRef, err := repo.Tag(gitArgs[0])
	if err != nil && err != git.ErrTagNotFound {
		return err
	} else if tagRef != nil {
		tag, err = repo.TagObject(tagRef.Hash())
		if err != nil {
			return err
		}
	}

	// If message is unset, use the tag's message if the tag already exists
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
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient)
		if err != nil {
			return errors.Wrapf(err, "failed to get next nonce")
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// If the APPNAME_REPONAME_PASS var is unset, set it to the user-defined push key pass.
	// This is required to allow git-sign learn the passphrase to unlock the push key.
	passVar := common.MakeRepoScopedEnvVar(cfg.GetAppName(), repo.GetName(), "PASS")
	if len(os.Getenv(passVar)) == 0 {
		os.Setenv(passVar, args.PushKeyPass)
	}

	// Check if the tag already exists and has previously been signed:
	// Skip resigning if push key of current attempt didn't change and only if args.ForceSign is false.
	if tag != nil && tag.PGPSignature != "" && !args.ForceSign {
		txd, _ := types.DecodeSignatureHeader([]byte(tag.PGPSignature))
		if txd != nil && txd.PushKeyID == pushKeyID {
			goto create_token
		}
	}

	// Skip signing if CreatePushTokenOnly is true
	if args.CreatePushTokenOnly {
		goto create_token
	}

	// Create the signed tag
	if err = repo.CreateTagWithMsg(gitArgs, args.Message, pushKeyID); err != nil {
		return err
	}

	// Create & set push request token on the remote in config.
create_token:

	// Skip push token creation if only reference signing was requested
	if args.SignRefOnly {
		return nil
	}

	// Create & set request token to remote URLs in config
	tagRef, _ = repo.Tag(gitArgs[0])
	if _, err = args.SetRemotePushToken(repo, &server.GenSetPushTokenArgs{
		TargetRemote: args.Remote,
		PushKey:      key,
		ResetTokens:  args.ResetTokens,
		Stderr:       args.Stderr,
		TxDetail: &types.TxDetail{
			Fee:       util.String(args.Fee),
			Value:     util.String(args.Value),
			Nonce:     args.Nonce,
			PushKeyID: pushKeyID,
			Reference: plumbing.NewTagReferenceName(gitFlags.Arg(0)).String(),
			Head:      tagRef.Hash().String(),
		},
	}); err != nil {
		return err
	}

	return nil
}

// populateSignTagArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignTagArgsFromRepoConfig(repo types.LocalRepo, args *SignTagArgs) {
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
