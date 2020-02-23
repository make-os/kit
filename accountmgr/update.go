package accountmgr

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gitlab.com/makeos/mosdef/util"
)

// UpdateCmd fetches and lists all accounts
func (am *AccountManager) UpdateCmd(addressOrIndex, passphrase string) error {

	if len(addressOrIndex) == 0 {
		return fmt.Errorf("Address is required")
	}

	// Unlock the account
	account, err := am.UIUnlockAccount(addressOrIndex, passphrase)
	if err != nil {
		return err
	}

	// Collect the new password
	fmt.Println("Enter your new password")
	newPassphrase, err := am.AskForPassword()
	if err != nil {
		return err
	}

	// Re-encrypt with the new passphrase
	newPassphraseHardened := hardenPassword([]byte(newPassphrase))
	updatedCipher, err := util.Encrypt(account.DecryptedCipher, newPassphraseHardened[:])
	if err != nil {
		return fmt.Errorf("unable to lock account with new passphrase")
	}

	filename := filepath.Join(am.accountDir, fmt.Sprintf("%d_%s", account.CreatedAt.Unix(), account.Address))
	err = ioutil.WriteFile(filename, updatedCipher, 0644)
	if err != nil {
		return fmt.Errorf("unable to write locked account to disk")
	}
	fmt.Println("Successfully updated account")

	return nil
}
