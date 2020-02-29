package account

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"

	funk "github.com/thoas/go-funk"
)

// ReadPassFromFile reads a passphrase from a file path; prints
// error messages to stdout
func ReadPassFromFile(path string) (string, error) {
	fullPath, _ := filepath.Abs(path)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		if funk.Contains(err.Error(), "no such file") {
			err = errors.Wrap(err, "password file not found")
		}
		if funk.Contains(err.Error(), "is a directory") {
			err = errors.Wrapf(err, "path is a directory. Expected a file")
		}
		return "", err
	}
	return strings.TrimSpace(strings.Trim(string(content), "/n")), nil
}

// RevealCmd decrypts an account and outputs the private key.
// If pass is provide and it is not a file path, it is used as
// the password. Otherwise, the file is read, trimmed of newline
// characters (left and right) and used as the password. When pass
// is set, interactive password collection is not used.
func (am *AccountManager) RevealCmd(addrOrIdx, pass string) error {

	if addrOrIdx == "" {
		return fmt.Errorf("address is required")
	}

	storedAcct, err := am.GetByIndexOrAddress(addrOrIdx)
	if err != nil {
		return err
	}

	fmt.Println(color.HiBlackString("Account: ") + storedAcct.Address)

	// if no password or password file is provided, ask for password
	var passphrase string
	if len(pass) == 0 {
		passphrase = am.AskForPasswordOnce()
		goto unlock
	}

	// If pass is not a path to a file, use pass as the passphrase.
	if !strings.HasPrefix(pass, "./") && !strings.HasPrefix(pass, "/") && filepath.Ext(pass) == "" {
		passphrase = pass
		goto unlock
	}

	// So, 'pass' contains a file path, read the password from it
	passphrase, err = ReadPassFromFile(pass)
	if err != nil {
		return err
	}

unlock:

	if err = storedAcct.Unlock(passphrase); err != nil {
		return errors.Wrap(err, "could not unlock account")
	}

	fmt.Println(color.HiCyanString("Private Key:"), storedAcct.key.PrivKey().Base58())

	return nil
}
