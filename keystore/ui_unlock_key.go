package keystore

import (
	"path/filepath"
	"strings"

	"github.com/themakeos/lobe/keystore/types"
	fmt2 "github.com/themakeos/lobe/util/colorfmt"
)

// UnlockKeyUI unlocks a target key by rendering prompt to collect a passphrase.
// addressOrIndex: The address or index of the key.
// passphrase: The user supplied passphrase. If not provided, an
// interactive session will be started to collect the passphrase
// Onsuccess, it returns the unlocked key and the passphrase used to unlock it.
func (ks *Keystore) UnlockKeyUI(addressOrIndex, passphrase, promptMsg string) (types.StoredKey, string, error) {
	var err error

	// Get the key
	storedAcct, err := ks.GetByIndexOrAddress(addressOrIndex)
	if err != nil {
		return nil, "", err
	}

	// Set default prompt if unset by caller
	if promptMsg == "" {
		promptMsg = fmt2.WhiteBoldString("Chosen Account: ") + storedAcct.GetUserAddress() + "\n"
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
		return nil, "", err
	}

unlock:
	if err = storedAcct.Unlock(passphrase); err != nil {
		return nil, "", err
	}

	return storedAcct, passphrase, nil
}
