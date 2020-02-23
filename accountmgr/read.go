package accountmgr

import (
	"fmt"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/btcsuite/btcutil/base58"
	funk "github.com/thoas/go-funk"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

var (
	// ErrAccountNotFound represents an error about a missing account
	ErrAccountNotFound = fmt.Errorf("account not found")
)

// StoredAccountMeta represents additional meta data of an account
type StoredAccountMeta map[string]interface{}

// StoredAccount represents an encrypted account stored on disk
type StoredAccount struct {

	// Address is the account's address
	Address string

	// Cipher includes the encryption data
	Cipher []byte

	// DecryptedCipher contains the decrypted data.
	// Only available after account is unlocked.
	DecryptedCipher []byte

	// key stores the instantiated equivalent of the stored account key
	key *crypto.Key

	// CreatedAt represents the time the account was created and stored on disk
	CreatedAt time.Time

	// Default indicates that this account is the default client account
	Default bool

	// Store other information about the account here
	meta StoredAccountMeta
}

// AccountExist checks if an account with a matching address exists
func (am *AccountManager) AccountExist(address string) (bool, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return false, err
	}

	for _, acct := range accounts {
		if acct.Address == address {
			return true, nil
		}
	}

	return false, nil
}

// GetDefault gets the default account
func (am *AccountManager) GetDefault() (*StoredAccount, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, ErrAccountNotFound
	}

	for _, a := range accounts {
		if a.Default {
			return a, nil
		}
	}

	return nil, ErrAccountNotFound
}

// GetByIndex returns an account by its current position in the
// list of accounts which is ordered by the time of creation.
func (am *AccountManager) GetByIndex(i int) (*StoredAccount, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	if acctLen := len(accounts); acctLen-1 < i {
		return nil, ErrAccountNotFound
	}

	return accounts[i], nil
}

// GetByIndexOrAddress gets an account by either its address or index
func (am *AccountManager) GetByIndexOrAddress(idxOrAddr string) (*StoredAccount, error) {
	if crypto.IsValidAddr(idxOrAddr) == nil {
		return am.GetByAddress(idxOrAddr)
	}
	if govalidator.IsNumeric(idxOrAddr) {
		idx, _ := strconv.Atoi(idxOrAddr)
		return am.GetByIndex(idx)
	}
	return nil, ErrAccountNotFound
}

// GetByAddress gets an account by its address in the list of accounts.
func (am *AccountManager) GetByAddress(addr string) (*StoredAccount, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	account := funk.Find(accounts, func(x *StoredAccount) bool {
		return x.Address == addr
	})

	if account == nil {
		return nil, ErrAccountNotFound
	}

	return account.(*StoredAccount), nil
}

// HasKey checks whether a key exist
func (sm StoredAccountMeta) HasKey(key string) bool {
	_, ok := sm[key]
	return ok
}

// Get returns a value
func (sm StoredAccountMeta) Get(key string) interface{} {
	return sm[key]
}

// GetMeta returns the meta information of the account
func (sa *StoredAccount) GetMeta() StoredAccountMeta {
	return sa.meta
}

// GetKey gets an instance of the decrypted account's key.
// Unlock() must be called first.
func (sa *StoredAccount) GetKey() *crypto.Key {
	return sa.key
}

// Unlock decrypts the account using the given passphrase.
// It populates the decrypted cipher and private key fields.
func (sa *StoredAccount) Unlock(passphrase string) error {

	passphraseBs := hardenPassword([]byte(passphrase))
	acctBytes, err := util.Decrypt(sa.Cipher, passphraseBs[:])
	if err != nil {
		if funk.Contains(err.Error(), "invalid key") {
			return fmt.Errorf("invalid passphrase")
		}
		return err
	}
	sa.DecryptedCipher = acctBytes

	// Decode from base58
	acctData, _, err := base58.CheckDecode(string(acctBytes))
	if err != nil {
		return fmt.Errorf("invalid passphrase")
	}

	// Decode from msgpack
	var accountData map[string]string
	if err := msgpack.Unmarshal(acctData, &accountData); err != nil {
		return fmt.Errorf("unable to parse account data")
	}

	// Convert the secret key to PrivKey object
	privKey, err := crypto.PrivKeyFromBase58(accountData["sk"])
	if err != nil {
		return err
	}
	sa.key = crypto.NewKeyFromPrivKey(privKey)

	return nil
}
