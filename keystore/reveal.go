package keystore

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	funk "github.com/thoas/go-funk"
)

// readPassFromFile reads a passphrase from a file path; prints
// error messages to stdout
func readPassFromFile(path string) (string, error) {
	fullPath, _ := filepath.Abs(path)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		if funk.Contains(err.Error(), "no such file") {
			err = errors.Wrap(err, "passphrase file not found")
		}
		if funk.Contains(err.Error(), "is a directory") {
			err = errors.Wrapf(err, "path is a directory. Expected a file")
		}
		return "", err
	}
	return strings.TrimSpace(strings.Trim(string(content), "/n")), nil
}

// RevealCmd decrypts a privKey and outputs the private privKey.
// If pass is provide and it is not a file path, it is used as
// the passphrase. Otherwise, the file contentis used as the
// passphrase. When pass is not set, the user is promoted to
// provide their passphrase.
func (ks *Keystore) RevealCmd(addrOrIdx, pass string) error {

	if addrOrIdx == "" {
		return fmt.Errorf("address is required")
	}

	storedAcct, err := ks.GetByIndexOrAddress(addrOrIdx)
	if err != nil {
		return err
	}

	var passphrase string
	if storedAcct.IsUnsafe() {
		pass = DefaultPassphrase
	}

	// if no passphrase or passphrase file is provided, ask for passphrase
	if len(pass) == 0 {
		passphrase = ks.AskForPasswordOnce()
		goto unlock
	}

	// If pass is not a path to a file, use pass as the passphrase.
	if !strings.HasPrefix(pass, "./") && !strings.HasPrefix(pass, "/") && filepath.Ext(pass) == "" {
		passphrase = pass
		goto unlock
	}

	// So, 'pass' contains a file path, read the passphrase from it
	passphrase, err = readPassFromFile(pass)
	if err != nil {
		return err
	}

unlock:

	if err = storedAcct.Unlock(passphrase); err != nil {
		return errors.Wrap(err, "could not unlock key")
	}

	fmt.Println(color.HiBlackString("Address: ") + storedAcct.GetAddress())
	fmt.Println(color.HiCyanString("Private Key:"), storedAcct.GetKey().PrivKey().Base58())

	return nil
}
