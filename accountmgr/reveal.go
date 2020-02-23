package accountmgr

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/util"

	"github.com/fatih/color"

	funk "github.com/thoas/go-funk"
)

// ReadPassFromFile reads a passphrase from a file path; prints
// error messages to stdout
func ReadPassFromFile(path string) (string, error) {

	fullPath, err := filepath.Abs(path)
	if err != nil {
		util.PrintCLIError("Invalid file path {%s}: %s", path, err.Error())
		return "", err
	}

	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		if funk.Contains(err.Error(), "no such file") {
			util.PrintCLIError("Password file {%s} not found.", path)
		}
		if funk.Contains(err.Error(), "is a directory") {
			util.PrintCLIError("Password file path {%s} is a directory. Expects a file.", path)
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

	var passphrase string

	if addrOrIdx == "" {
		return fmt.Errorf("address is required")
	}

	storedAcct, err := am.GetByIndexOrAddress(addrOrIdx)
	if err != nil {
		return err
	}

	fmt.Println(color.HiBlackString("Account: ") + storedAcct.Address)

	// if no password or password file is provided, ask for password
	if len(pass) == 0 {
		passphrase, err = am.AskForPasswordOnce()
		if err != nil {
			return err
		}
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
		return errors.Wrap(err, "Could not unlock account")
	}

	fmt.Println(color.HiCyanString("Private Key:"), storedAcct.key.PrivKey().Base58())

	return nil
}
