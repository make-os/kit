package signcmd

import (
	"strconv"

	"github.com/make-os/kit/cmd/common"
	types3 "github.com/make-os/kit/cmd/signcmd/types"
	"github.com/make-os/kit/config"
	types2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/util"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

// SignTagCmd create and sign a push token for a given tag.
func SignTagCmd(cfg *config.AppConfig, cmdArg []string, repo types2.LocalRepo, args *types3.SignTagArgs) error {

	populateSignTagArgsFromRepoConfig(repo, args)

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

	// Get the tag object
	tagRef, err := repo.Tag(cmdArg[0])
	if err != nil {
		return err
	}

	// Get the next nonce, if not set
	if args.Nonce == 0 {
		nonce, err := args.GetNextNonce(pushKeyID, args.RPCClient)
		if err != nil {
			return errors.Wrapf(err, "failed to get next nonce")
		}
		args.Nonce, _ = strconv.ParseUint(nonce, 10, 64)
	}

	// Create & apply request token to the remote
	if err = args.CreateApplyPushTokenToRemote(repo, &server.MakeAndApplyPushTokenToRemoteArgs{
		TargetRemote: args.Remote,
		PushKey:      key,
		ResetTokens:  args.ResetTokens,
		Stderr:       args.Stderr,
		TxDetail: &types.TxDetail{
			Fee:       util.String(args.Fee),
			Value:     util.String(args.Value),
			Nonce:     args.Nonce,
			PushKeyID: pushKeyID,
			Reference: tagRef.Name().String(),
			Head:      tagRef.Hash().String(),
		},
	}); err != nil {
		return err
	}

	return nil
}

// populateSignTagArgsFromRepoConfig populates empty arguments field from repo config.
func populateSignTagArgsFromRepoConfig(repo types2.LocalRepo, args *types3.SignTagArgs) {
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
