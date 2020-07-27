package keystore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/util/crypto"
)

// UpdateCmd fetches and lists all accounts
func (ks *Keystore) UpdateCmd(addressOrIndex, passphrase string) error {

	if len(addressOrIndex) == 0 {
		return fmt.Errorf("address or address index is required")
	}

	// Unlock the key
	acct, err := ks.UIUnlockKey(addressOrIndex, passphrase, "")
	if err != nil {
		return err
	}

	// Collect the new passphrase
	fmt.Fprintln(ks.out, "Enter your new passphrase")
	newPassphrase, err := ks.AskForPassword()
	if err != nil {
		return err
	}

	// Re-encrypt with the new passphrase
	newPassphraseHardened := hardenPassword([]byte(newPassphrase))
	updatedCipher, err := crypto.Encrypt(acct.GetUnlockedData(), newPassphraseHardened[:])
	if err != nil {
		return fmt.Errorf("unable to re-lock key")
	}

	// Backup the existing key file
	backupPath := filepath.Join(ks.dir, acct.GetFilename()) + "_backup"
	os.Rename(filepath.Join(ks.dir, acct.GetFilename()), backupPath)

	// Create the new key file
	unprotected := passphrase == DefaultPassphrase
	fileID := types.KeyTypeChar[acct.GetType()] + acct.GetKey().PubKey().Base58()
	fileName := makeKeyStoreName(acct.GetCreatedAt().Unix(), fileID, unprotected)
	err = ioutil.WriteFile(filepath.Join(ks.dir, fileName), updatedCipher, 0644)
	if err != nil {
		return fmt.Errorf("unable to write payload to disk")
	}

	// Delete the backup
	os.RemoveAll(backupPath)

	fmt.Fprintln(ks.out, "Successfully updated key")

	return nil
}
