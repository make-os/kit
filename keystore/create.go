package keystore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/logrusorgru/aurora"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/util"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	crypto2 "github.com/make-os/kit/util/crypto"
	"github.com/pkg/errors"
)

const (
	DefaultPassphrase = "passphrase"
)

// CreateKey creates a new key
func (ks *Keystore) CreateKey(key *ed25519.Key, keyType types.KeyType, passphrase string) error {

	// Check whether the key already exists. Return error if true.
	exist, err := ks.Exist(key.Addr().String())
	if err != nil {
		return err
	} else if exist {
		return fmt.Errorf("key already exists")
	}

	// When no passphrase is provided, we use a default passphrase
	// which will cause the key to be flagged as 'unprotected'.
	if passphrase == "" {
		passphrase = DefaultPassphrase
	}

	// Harden the passphrase
	passphraseHardened := hardenPassword([]byte(passphrase))

	// Create the serialized key payload
	keyData := util.ToBytes(types.KeyPayload{
		SecretKey:     key.PrivKey().Base58(),
		Type:          int(keyType),
		FormatVersion: Version,
	})

	// Encode the key payload with base58 checksum enabled and encrypt
	b58AcctBs := base58.CheckEncode(keyData, 1)
	ct, err := crypto2.Encrypt([]byte(b58AcctBs), passphraseHardened[:])
	if err != nil {
		return errors.Wrap(err, "key encryption failed")
	}

	// Save the key
	now := time.Now().Unix()
	unprotected := passphrase == DefaultPassphrase
	fileName := makeKeyStoreName(now, types.KeyTypeChar[keyType]+key.PubKey().Base58(), unprotected)
	err = ioutil.WriteFile(filepath.Join(ks.dir, fileName), ct, 0644)
	if err != nil {
		return err
	}

	return nil
}

// makeKeyStoreName creates a name to be used as a keystore file name
func makeKeyStoreName(timeNow int64, fileID string, unprotected bool) string {
	fn := fmt.Sprintf("%d_%s", timeNow, fileID)
	if unprotected {
		fn = fn + "_unprotected"
	}
	return fn
}

// CreateCmd creates a new key in the keystore.
// It will prompt the user to obtain encryption passphrase if one is not provided.
// If seed is non-zero, it is used as the seed for key generation, otherwise,
// one will be randomly generated.
// If passphrase is a file path, the file is read and its content is used as the
// passphrase.
// If nopass is true, the default encryption passphrase is
// used and the key will be marked 'unprotected'
func (ks *Keystore) CreateCmd(
	keyType types.KeyType,
	seed int64,
	passphrase string,
	nopass bool) (*ed25519.Key, error) {

	var passFromPrompt string
	var err error

	// If no passphrase is provided, start an interactive session to
	// collect the passphrase
	if !nopass && strings.TrimSpace(passphrase) == "" {
		fmt.Fprintln(ks.out, "Your new key needs to be locked. Please enter a passphrase.")
		passFromPrompt, err = ks.AskForPassword()
		if err != nil {
			return nil, err
		}
	}

	// But if the passphrase is set and is a valid file, read it and use as passphrase
	if len(passphrase) > 0 && (os.IsPathSeparator(passphrase[0]) || (len(passphrase) >= 2 && passphrase[:2] == "./")) {
		content, err := ioutil.ReadFile(passphrase)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read passphrase file")
		}
		passFromPrompt = strings.TrimSpace(strings.Trim(string(content), "/n"))
	} else if len(passphrase) > 0 {
		passFromPrompt = passphrase
	}

	// Generate a key
	key, err := ed25519.NewKey(nil)
	if seed != 0 {
		key, err = ed25519.NewKey(&seed)
	}
	if err != nil {
		return nil, err
	}

	// Create the key
	if err := ks.CreateKey(key, keyType, passFromPrompt); err != nil {
		return nil, err
	}

	fmt.Fprintln(ks.out, fmt2.NewColor(aurora.Green, aurora.Bold).Sprint("✅ Key successfully created!"))
	if keyType == types.KeyTypeUser {
		fmt.Fprintln(ks.out, " - Address:", fmt2.CyanString(key.Addr().String()))
	} else if keyType == types.KeyTypePush {
		fmt.Fprintln(ks.out, " - Address:", fmt2.CyanString(key.PushAddr().String()))
	}

	return key, nil
}
