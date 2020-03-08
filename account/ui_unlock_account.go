package account

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"gitlab.com/makeos/mosdef/types/core"
)

// UIUnlockAccount renders a CLI UI to unlock a target account.
// addressOrIndex: The address or index of the account.
// passphrase: The user supplied passphrase. If not provided, an
// interactive session will be started to collect the passphrase
func (am *AccountManager) UIUnlockAccount(addressOrIndex, passphrase string) (core.StoredAccount, error) {

	var err error

	// Get the account.
	storedAcct, err := am.GetByIndexOrAddress(addressOrIndex)
	if err != nil {
		return nil, err
	}

	fmt.Println(color.HiBlackString("Chosen Account: ") + storedAcct.GetAddress())

	// Ask for passphrase is not provided
	if passphrase == "" {
		passphrase = am.AskForPasswordOnce()
	}

	// If passphrase is not a path to a file, proceed to unlock the account
	if !strings.HasPrefix(passphrase, "./") &&
		!strings.HasPrefix(passphrase, "/") &&
		filepath.Ext(passphrase) == "" {
		goto unlock
	}

	// So, 'passphrase' contains a file path, read the password from it
	passphrase, err = ReadPassFromFile(passphrase)
	if err != nil {
		return nil, err
	}

unlock:
	if err = storedAcct.Unlock(passphrase); err != nil {
		return nil, err
	}

	return storedAcct, nil
}
