package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/keystore/types"
	remotetypes "gitlab.com/makeos/mosdef/remote/types"
)

// KeyUnlocker describes a function for unlocking a keystore key.
type KeyUnlocker func(
	cfg *config.AppConfig,
	keyAddrOrIdx,
	defaultPassphrase string,
	targetRepo remotetypes.LocalRepo) (types.StoredKey, error)

// UnlockKey takes a key address or index, unlocks it and returns the key.
// - It will using the given passphrase if set, otherwise
// - if the target repo is set, it will try to get it from the git config (user.passphrase).
// - If passphrase is still unknown, it will attempt to get it from an environment variable.
func UnlockKey(
	cfg *config.AppConfig,
	keyAddrOrIdx,
	passphrase string,
	targetRepo remotetypes.LocalRepo) (types.StoredKey, error) {

	// Get the key from the key store
	ks := keystore.New(cfg.KeystoreDir())
	ks.SetOutput(ioutil.Discard)

	// Ensure the key exist
	key, err := ks.GetByIndexOrAddress(keyAddrOrIdx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find key (%s)", keyAddrOrIdx)
	}

	// If passphrase is unset and target repo is set, attempt to get the
	// passphrase from 'user.passphrase' config.
	if !key.IsUnprotected() && passphrase == "" && targetRepo != nil {
		repoCfg, err := targetRepo.Config()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get repo config")
		}
		passphrase = repoCfg.Raw.Section("user").Option("passphrase")

		// If we still don't have a passphrase, get it from the repo scoped env variable.
		if passphrase == "" {
			passphrase = os.Getenv(MakeRepoScopedPassEnvVar(config.AppName, targetRepo.GetName()))
		}
	}

	// If key is protected and still no passphrase,
	// try to get it from the general passphrase env variable
	if !key.IsUnprotected() && passphrase == "" {
		passphrase = os.Getenv(MakePassEnvVar(config.AppName))
	}

	// If key is protected and still no passphrase, exit with error
	if !key.IsUnprotected() && passphrase == "" {
		return nil, fmt.Errorf("passphrase of signing key is required")
	}

	key, err = ks.UIUnlockKey(keyAddrOrIdx, passphrase)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unlock key (%s)", keyAddrOrIdx)
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
