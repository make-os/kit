package keystore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gitlab.com/makeos/mosdef/util"
)

// UpdateCmd fetches and lists all accounts
func (ks *Keystore) UpdateCmd(addressOrIndex, passphrase string) error {

	if len(addressOrIndex) == 0 {
		return fmt.Errorf("Address or address index is required")
	}

	// Unlock the keystore
	account, err := ks.UIUnlockAccount(addressOrIndex, passphrase)
	if err != nil {
		return err
	}

	// Collect the new passphrase
	fmt.Println("Enter your new passphrase")
	newPassphrase, err := ks.AskForPassword()
	if err != nil {
		return err
	}

	// Re-encrypt with the new passphrase
	newPassphraseHardened := hardenPassword([]byte(newPassphrase))
	updatedCipher, err := util.Encrypt(account.GetUnlockedData(), newPassphraseHardened[:])
	if err != nil {
		return fmt.Errorf("unable to relock keystore")
	}

	// Backup the existing key file
	backupPath := filepath.Join(ks.dir, account.GetFilename()) + "_backup"
	os.Rename(filepath.Join(ks.dir, account.GetFilename()), backupPath)

	// Create the new key file
	filename := createKeyFileName(account.GetCreatedAt().Unix(), account.GetAddress(), newPassphrase)
	err = ioutil.WriteFile(filepath.Join(ks.dir, filename), updatedCipher, 0644)
	if err != nil {
		return fmt.Errorf("unable to write relocked keystore to disk")
	}

	// Delete the backup
	os.RemoveAll(backupPath)

	fmt.Println("Successfully updated keystore")

	return nil
}
