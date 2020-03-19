package keystore

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/core"
)

// ImportCmd creates a new key from a private key stored in a file.
// If pass is provide and it is not a file path, it is used as
// the passphrase. Otherwise, the file is read and used as the
// passphrase. When pass is not set, the user is prompted to
// provide the passphrase.
func (ks *Keystore) ImportCmd(keyfile string, keyType core.KeyType, pass string, out io.Writer) error {

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
		fmt.Fprintln(out, "Your new account needs to be locked with a passphrase. Please enter a passphrase.")
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

	fmt.Fprintln(out, "Import successful. New key created, encrypted and stored")
	if keyType == core.KeyTypeAccount {
		fmt.Fprintln(out, "Address:", color.CyanString(key.Addr().String()))
	} else if keyType == core.KeyTypePush {
		fmt.Fprintln(out, "Address:", color.CyanString(key.PushAddr().String()))
	}

	return nil
}
