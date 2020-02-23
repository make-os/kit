package accountmgr

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/pkg/errors"
	funk "github.com/thoas/go-funk"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// CreateAccount creates a new account
func (am *AccountManager) CreateAccount(defaultAccount bool, address *crypto.Key,
	passphrase string) error {

	if address == nil {
		return fmt.Errorf("Address is required")
	} else if passphrase == "" {
		return fmt.Errorf("Passphrase is required")
	}

	exist, err := am.AccountExist(address.Addr().String())
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("An account with a matching seed already exist")
	}

	// hash passphrase to get 32 bit encryption key
	passphraseHardened := hardenPassword([]byte(passphrase))

	// construct, json encode and encrypt account data
	acctDataBs, _ := msgpack.Marshal(map[string]string{
		"addr": address.Addr().String(),
		"sk":   address.PrivKey().Base58(),
		"pk":   address.PubKey().Base58(),
		"v":    accountEncryptionVersion,
	})

	// base58 check encode
	b58AcctBs := base58.CheckEncode(acctDataBs, 1)

	ct, err := util.Encrypt([]byte(b58AcctBs), passphraseHardened[:])
	if err != nil {
		return err
	}

	// Persist encrypted account data
	now := time.Now().Unix()
	fileName := path.Join(am.accountDir, fmt.Sprintf("%d_%s", now, address.Addr()))
	if defaultAccount {
		fileName = path.Join(am.accountDir, fmt.Sprintf("%d_%s_default", now, address.Addr()))
	}

	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(ct)
	if err != nil {
		return err
	}

	return nil
}

// CreateCmd creates a new account and interactively obtains encryption passphrase.
// defaultAccount argument indicates that this account should be marked as default.
// If seed is non-zero, it is used. Otherwise, one will be randomly generated.
// If pass is provide and it is not a file path, it is used as
// the password. Otherwise, the file is read, trimmed of newline
// characters (left and right) and used as the password. When pass
// is set, interactive password collection is not used.
func (am *AccountManager) CreateCmd(defaultAccount bool, seed int64, pass string) (*crypto.Key, error) {

	var passphrase string
	var err error

	// If no password is provided, we start an interactive session to
	// collect the password or passphrase
	if len(pass) == 0 {
		fmt.Println("Your new account needs to be locked with a password. Please enter a password.")
		passphrase, err = am.AskForPassword()
		if err != nil {
			return nil, err
		}
	}

	// But if the password is set and is a valid file, read it and use as password
	if len(pass) > 0 && (os.IsPathSeparator(pass[0]) || (len(pass) >= 2 && pass[:2] == "./")) {
		content, err := ioutil.ReadFile(pass)
		if err != nil {
			if funk.Contains(err.Error(), "no such file") {
				err = errors.Wrapf(err, "Password file {%s} not found.", pass)
			}
			if funk.Contains(err.Error(), "is a directory") {
				err = errors.Wrapf(err, "Password file path {%s} is a directory. Expects a file.", pass)
			}
			return nil, err
		}
		passphrase = string(content)
		passphrase = strings.TrimSpace(strings.Trim(passphrase, "/n"))
	} else if len(pass) > 0 {
		passphrase = pass
	}

	// Generate an address (which includes a private key)
	var address *crypto.Key
	address, err = crypto.NewKey(nil)
	if seed != 0 {
		address, err = crypto.NewKey(&seed)
	}

	if err != nil {
		return nil, err
	}

	// Create and encrypted the account on disk
	if err := am.CreateAccount(defaultAccount, address, passphrase); err != nil {
		return nil, err
	}

	return address, nil
}
