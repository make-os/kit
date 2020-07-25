package keystore

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/keystore/types"
	fmt2 "gitlab.com/makeos/lobe/util/colorfmt"
)

// ImportCmd creates a new key from a private key stored in a file.
// If pass is provide and it is not a file path, it is used as
// the passphrase. Otherwise, the file is read and used as the
// passphrase. When pass is not set, the user is prompted to
// provide the passphrase.
func (ks *Keystore) ImportCmd(keyfile string, keyType types.KeyType, pass string) error {

	if keyfile == "" {
		return fmt.Errorf("key file path is required")
	}

	fullKeyFilePath, err := filepath.Abs(keyfile)
	if err != nil {
		return fmt.Errorf("invalid key file path {%s}", keyfile)
	}

	keyFileContent, err := ioutil.ReadFile(fullKeyFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to read key file")
	}

	// Ensure the key file contains a valid private key
	fileContentStr := strings.TrimSpace(string(keyFileContent))
	sk, err := crypto.PrivKeyFromBase58(fileContentStr)
	if err != nil {
		return errors.Wrap(err, "key file contains invalid private key")
	}

	var content []byte

	// If no passphrase or passphrase file is provided, ask for passphrase
	passphrase := ""
	if len(pass) == 0 {
		fmt.Fprintln(ks.out, "Your new account needs to be locked. Please enter a passphrase.")
		passphrase, err = ks.AskForPassword()
		if err != nil {
			return err
		}
		goto create
	}

	if !strings.HasPrefix(pass, "./") && !strings.HasPrefix(pass, "/") && filepath.Ext(pass) == "" {
		passphrase = pass
		goto create
	}

	content, err = ioutil.ReadFile(pass)
	if err != nil {
		return errors.Wrap(err, "failed to read passphrase")
	}
	passphrase = string(content)
	passphrase = strings.TrimSpace(strings.Trim(passphrase, "/n"))

create:
	key := crypto.NewKeyFromPrivKey(sk)
	if err := ks.CreateKey(key, keyType, passphrase); err != nil {
		return err
	}

	fmt.Fprintln(ks.out, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Key imported successfully!"))
	if keyType == types.KeyTypeAccount {
		fmt.Fprintln(ks.out, " - Address:", fmt2.CyanString(key.Addr().String()))
	} else if keyType == types.KeyTypePush {
		fmt.Fprintln(ks.out, " - Address:", fmt2.CyanString(key.PushAddr().String()))
	}

	return nil
}
