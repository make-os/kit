package keystore

import (
	"path/filepath"
	"strings"

	"gitlab.com/makeos/lobe/keystore/types"
	fmt2 "gitlab.com/makeos/lobe/util/colorfmt"
)

// UIUnlockKey renders a CLI UI to unlock a target key..
// addressOrIndex: The address or index of the key.
// passphrase: The user supplied passphrase. If not provided, an
// interactive session will be started to collect the passphrase
func (ks *Keystore) UIUnlockKey(addressOrIndex, passphrase, promptMsg string) (types.StoredKey, error) {
	var err error

	// Get the key
	storedAcct, err := ks.GetByIndexOrAddress(addressOrIndex)
	if err != nil {
		return nil, err
	}

	// Set default prompt if unset by caller
	if promptMsg == "" {
		promptMsg = fmt2.WhiteBoldString("Chosen Account: ") + storedAcct.GetAddress() + "\n"
	}

	// Set the passphrase to the default passphrase if account
	// is encrypted with unprotected passphrase
	if storedAcct.IsUnprotected() {
		passphrase = DefaultPassphrase
	}

	// Ask for passphrase if unset
	if passphrase == "" {
		passphrase = ks.AskForPasswordOnce(promptMsg)
	}

	// If passphrase is not a path to a file, proceed to unlock the key.
	if !strings.HasPrefix(passphrase, "./") && !strings.HasPrefix(passphrase, "/") &&
		filepath.Ext(passphrase) == "" {
		goto unlock
	}

	// So, 'passphrase' contains a file path, read the passphrase from it
	passphrase, err = readPassFromFile(passphrase)
	if err != nil {
		return nil, err
	}

unlock:
	if err = storedAcct.Unlock(passphrase); err != nil {
		return nil, err
	}

	return storedAcct, nil
}
