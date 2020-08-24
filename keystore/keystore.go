// Package keystore provides key creation and management functionalities.
package keystore

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/make-os/lobe/util/colorfmt"
	io2 "github.com/make-os/lobe/util/io"
	"golang.org/x/crypto/scrypt"
)

var (
	Version = "0.0.1"
)

// promptFunc represents a function that can collect user input
type promptFunc func(string, ...interface{}) (string, error)

// Keystore implements Keystore. It provides the ability to
// create, update, fetch and import keys and accounts.
type Keystore struct {
	dir         string
	getPassword promptFunc
	out         io.Writer
}

// New creates an instance of Keystore.
// dir is where encrypted key files are stored.
// EXPECTS:
// - dir to have been created
func New(dir string) *Keystore {
	am := new(Keystore)
	am.dir = dir
	am.getPassword = func(s string, args ...interface{}) (string, error) {
		s = fmt.Sprintf("\033[33m%s>\033[0m ", s)
		return io2.ReadInput(fmt.Sprintf(s, args...), &io2.InputReaderArgs{Password: true})
	}
	am.out = os.Stdout
	return am
}

// SetOutput sets the output writer
func (ks *Keystore) SetOutput(out io.Writer) {
	ks.out = out
}

// AskForPassword starts an interactive prompt to collect passphrase.
// Returns error if passphrase and repeated passphrases do not match
func (ks *Keystore) AskForPassword(prompt ...string) (string, error) {

	// Set and print prompt.
	// If prompt is passed in, use it instead of the default
	promptStr := ""
	if len(prompt) > 0 {
		promptStr = prompt[0]
	}
	fmt.Fprint(ks.out, colorfmt.BoldString(promptStr))

	for {
		passphrase, err := ks.getPassword("Passphrase")
		if err != nil {
			return "", err
		}
		if len(passphrase) == 0 {
			continue
		}

		passphraseRepeat, err := ks.getPassword("Repeat Passphrase")
		if err != nil {
			return "", err
		}
		if passphrase != passphraseRepeat {
			return "", fmt.Errorf("passphrases did not match")
		}

		return passphrase, nil
	}
}

// AskForPasswordOnce is like askForPassword but it does not
// ask to confirm passphrase.
func (ks *Keystore) AskForPasswordOnce(prompt ...string) (string, error) {

	// Set and print prompt.
	// If prompt is passed in, use it instead of the default
	promptStr := "Enter your passphrase to unlock the key\n"
	if len(prompt) > 0 {
		promptStr = prompt[0]
	}
	fmt.Fprint(ks.out, colorfmt.BoldString(promptStr))

	for {
		passphrase, err := ks.getPassword("Passphrase")
		if err != nil {
			return "", err
		}
		if len(passphrase) == 0 {
			continue
		}
		return passphrase, nil
	}
}

// harden the given passphrase using scrypt
func hardenPassword(pass []byte) []byte {
	passHash := sha256.Sum256(pass)
	var salt = passHash[16:]
	newPass, err := scrypt.Key(pass, salt, 32768, 8, 1, 32)
	if err != nil {
		panic(err)
	}
	return newPass
}
