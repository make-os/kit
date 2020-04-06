package keystore

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
)

// readPassFromFile reads a passphrase from a file path; prints
// error messages to stdout
func readPassFromFile(path string) (string, error) {
	fullPath, _ := filepath.Abs(path)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", errors.Wrap(err, "unable to read passphrase file")
	}
	return strings.TrimSpace(strings.Trim(string(content), "/n")), nil
}

// RevealCmd decrypts a key and prints the private key.
// If pass is provide and it is not a file path, it is used as
// the passphrase. Otherwise, the file content is used as the
// passphrase. When pass is not set, the user is prompted to
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
	if storedAcct.IsUnprotected() {
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

	fmt.Fprintln(ks.out, color.HiBlackString("Address: ")+storedAcct.GetAddress())
	fmt.Fprintln(ks.out, color.HiBlackString("Public Key: ")+storedAcct.GetKey().PubKey().Base58())
	fmt.Fprintln(ks.out, color.HiCyanString("Private Key:"), storedAcct.GetKey().PrivKey().Base58())

	return nil
}
