package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/keystore/types"
	types3 "gitlab.com/makeos/mosdef/remote/pushpool/types"
)

// PushKeyUnlocker describes a function for fetching and unlocking a push key from the keystore
type PushKeyUnlocker func(
	cfg *config.AppConfig,
	pushKeyID,
	defaultPassphrase string,
	targetRepo types3.LocalRepo) (types.StoredKey, error)

// UnlockPushKey takes a push key ID and unlocks it using the default passphrase
// or one obtained from the git config of the repository or from an environment variable.
func UnlockPushKey(
	cfg *config.AppConfig,
	pushKeyID,
	defaultPassphrase string,
	targetRepo types3.LocalRepo) (types.StoredKey, error) {

	// Get the push key from the key store
	ks := keystore.New(cfg.KeystoreDir())
	ks.SetOutput(ioutil.Discard)

	// Ensure the push key exist
	key, err := ks.GetByIndexOrAddress(pushKeyID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find push key (%s)", pushKeyID)
	}

	// Get the request token from the config
	repoCfg, err := targetRepo.Config()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get repo config")
	}

	// APPNAME_REPONAME_PASS may contain the passphrase
	_, repoName := filepath.Split(targetRepo.GetPath())
	passEnvVarName := fmt.Sprintf("%s_%s_PASS", strings.ToUpper(config.AppName), strings.ToUpper(repoName))

	// If push key is protected, require passphrase
	var passphrase = defaultPassphrase
	if !key.IsUnprotected() && passphrase == "" {

		// Get the password from the "user.passphrase" option in git config
		passphrase = repoCfg.Raw.Section("user").Option("passphrase")

		// If we still don't have a passphrase, get it from the env variable:
		// APPNAME_REPONAME_PASS
		if passphrase == "" {
			passphrase = os.Getenv(passEnvVarName)
		}

		// Well, if no passphrase still, so exit with error
		if passphrase == "" {
			return nil, fmt.Errorf("passphrase of signing key is required")
		}
	}

	key, err = ks.UIUnlockKey(pushKeyID, passphrase)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unlock push key (%s)", pushKeyID)
	}

	// Set the passphrase on the env var so signing/verify commands
	// can learn about the passphrase
	os.Setenv(passEnvVarName, passphrase)

	return key, nil
}
