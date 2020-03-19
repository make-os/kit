// Package keystore provides key creation and management functionalities.
package keystore

import (
	"crypto/sha256"
	"fmt"

	"github.com/fatih/color"
	"golang.org/x/crypto/scrypt"

	"github.com/ellcrys/go-prompt"
)

var (
	Version = "0.0.1"
)

// promptFunc represents a function that can collect user input
type promptFunc func(string, ...interface{}) string

// Keystore defines functionalities to create, update, fetch and import accounts.
type Keystore struct {
	dir         string
	getPassword promptFunc
}

// New creates an instance of Keystore.
// dir is where encrypted key files are stored.
// EXPECTS:
// - dir to have been created
func New(dir string) *Keystore {
	am := new(Keystore)
	am.dir = dir
	am.getPassword = prompt.Password
	return am
}

// AskForPassword starts an interactive prompt to collect passphrase.
// Returns error if passphrase and repeated passphrases do not match
func (ks *Keystore) AskForPassword() (string, error) {
	for {
		passphrase := ks.getPassword("Passphrase")
		if len(passphrase) == 0 {
			continue
		}

		passphraseRepeat := ks.getPassword("Repeat Passphrase")
		if passphrase != passphraseRepeat {
			return "", fmt.Errorf("Passphrases did not match")
		}

		return passphrase, nil
	}
}

// AskForPasswordOnce is like askForPassword but it does not
// ask to confirm passphrase.
func (ks *Keystore) AskForPasswordOnce() string {
	fmt.Println(color.CyanString("Enter your passphrase to unlock the key"))
	for {
		passphrase := ks.getPassword("Passphrase")
		if len(passphrase) == 0 {
			continue
		}
		return passphrase
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
