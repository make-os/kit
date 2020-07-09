package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/keystore/types"
	remotetypes "gitlab.com/makeos/mosdef/remote/types"
)

// UnlockKeyArgs contains arguments for UnlockKey
type UnlockKeyArgs struct {
	KeyAddrOrIdx string
	AskPass      bool
	Passphrase   string
	TargetRepo   remotetypes.LocalRepo
	Prompt       string
	Stdout       io.Writer
}

// KeyUnlocker describes a function for unlocking a keystore key.
type KeyUnlocker func(cfg *config.AppConfig, args *UnlockKeyArgs) (types.StoredKey, error)

// UnlockKey takes a key address or index, unlocks it and returns the key.
// - It will using the given passphrase if set, otherwise
// - if the target repo is set, it will try to get it from the git config (user.passphrase).
// - If passphrase is still unknown, it will attempt to get it from an environment variable.
func UnlockKey(cfg *config.AppConfig, args *UnlockKeyArgs) (types.StoredKey, error) {

	// Get the key from the key store
	ks := keystore.New(cfg.KeystoreDir())
	if args.Stdout != nil {
		ks.SetOutput(args.Stdout)
	}

	// Get the key by address or index
	key, err := ks.GetByIndexOrAddress(args.KeyAddrOrIdx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find key (%s)", args.KeyAddrOrIdx)
	}

	// If passphrase is unset and target repo is set, attempt to get the
	// passphrase from 'user.passphrase' config.
	unprotected := key.IsUnprotected()
	if !unprotected && args.Passphrase == "" && args.TargetRepo != nil {
		repoCfg, err := args.TargetRepo.Config()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get repo config")
		}
		args.Passphrase = repoCfg.Raw.Section("user").Option("passphrase")

		// If we still don't have a passphrase, get it from the repo scoped env variable.
		if args.Passphrase == "" {
			args.Passphrase = os.Getenv(MakeRepoScopedPassEnvVar(config.AppName, args.TargetRepo.GetName()))
		}
	}

	// If key is protected and still no passphrase,
	// try to get it from the general passphrase env variable
	if !unprotected && args.Passphrase == "" {
		args.Passphrase = os.Getenv(MakePassEnvVar(config.AppName))
	}

	// If key is protected and still no passphrase, exit with error
	if !unprotected && args.Passphrase == "" && !args.AskPass {
		return nil, fmt.Errorf("passphrase of signing key is required")
	}

	key, err = ks.UIUnlockKey(args.KeyAddrOrIdx, args.Passphrase, args.Prompt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unlock key (%s)", args.KeyAddrOrIdx)
	}

	return key, nil
}

// MakeRepoScopedPassEnvVar returns a repo-specific env variable
// expected to contain passphrase for unlocking an account.
func MakeRepoScopedPassEnvVar(appName, repoName string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_PASS", appName, repoName))
}

// MakePassEnvVar is the name of the env variable expected to contain a key's passphrase.
func MakePassEnvVar(appName string) string {
	return strings.ToUpper(fmt.Sprintf("%s_PASS", appName))
}
