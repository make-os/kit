package accountmgr

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/crypto"

	funk "github.com/thoas/go-funk"
)

// ImportCmd takes a keyfile containing unencrypted password to create
// a new account. Keyfile must be a path to a file that exists.
// If pass is provide and it is not a file path, it is used as
// the password. Otherwise, the file is read, trimmed of newline
// characters (left and right) and used as the password. When pass
// is set, interactive password collection is not used.
func (am *AccountManager) ImportCmd(keyFile, pass string) error {

	if keyFile == "" {
		return fmt.Errorf("Key file is required")
	}

	fullKeyFilePath, err := filepath.Abs(keyFile)
	if err != nil {
		return fmt.Errorf("Invalid keyFile path {%s}", keyFile)
	}

	keyFileContent, err := ioutil.ReadFile(fullKeyFilePath)
	if err != nil {
		if funk.Contains(err.Error(), "no such file") {
			err = errors.Wrapf(err, "Key file {%s} not found.", keyFile)
		}
		if funk.Contains(err.Error(), "is a directory") {
			err = errors.Wrapf(err, "Key file {%s} is a directory. Expects a file.", keyFile)
		}
		return err
	}

	// Attempt to validate and instantiate the private key
	fileContentStr := strings.TrimSpace(string(keyFileContent))
	sk, err := crypto.PrivKeyFromBase58(fileContentStr)
	if err != nil {
		return errors.Wrap(err, "Key file contains invalid private key")
	}

	var content []byte

	// if no password or password file is provided, ask for password
	passphrase := ""
	if len(pass) == 0 {
		fmt.Println("Your new account needs to be locked with a password. Please enter a password.")
		passphrase, err = am.AskForPassword()
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
		if funk.Contains(err.Error(), "no such file") {
			err = errors.Wrapf(err, "Password file {%s} not found.", pass)
		}
		if funk.Contains(err.Error(), "is a directory") {
			err = errors.Wrapf(err, "Password file path {%s} is a directory. Expects a file.", pass)
		}
		return err
	}
	passphrase = string(content)
	passphrase = strings.TrimSpace(strings.Trim(passphrase, "/n"))

create:
	address := crypto.NewKeyFromPrivKey(sk)
	if err := am.CreateAccount(false, address, passphrase); err != nil {
		return err
	}

	fmt.Println("Import successful. New account created, encrypted and stored")
	fmt.Println("Address:", color.CyanString(address.Addr().String()))

	return nil
}
