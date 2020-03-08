package account

import (
	"fmt"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/btcsuite/btcutil/base58"
	funk "github.com/thoas/go-funk"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// StoredAccount represents an encrypted account stored on disk
type StoredAccount struct {

	// Address is the account's address
	Address string

	// Cipher includes the encryption data
	Cipher []byte

	// Data contains the decrypted data.
	// Only available after account is unlocked.
	Data []byte

	// key stores the instantiated equivalent of the stored account key
	key *crypto.Key

	// CreatedAt represents the time the account was created and stored on disk
	CreatedAt time.Time

	// Default indicates that this account is the default client account
	Default bool

	// Store other information about the account here
	meta core.StoredAccountMeta
}

// GetMeta returns the meta information of the account
func (sa *StoredAccount) GetMeta() core.StoredAccountMeta {
	return sa.meta
}

// IsDefault checks whether an account is the default
func (sa *StoredAccount) IsDefault() bool {
	return sa.Default
}

// GetAddress returns the address of the account
func (sa *StoredAccount) GetAddress() string {
	return sa.Address
}

// GetKey gets an instance of the decrypted account's key.
// Unlock() must be called first.
func (sa *StoredAccount) GetKey() *crypto.Key {
	return sa.key
}

// GetUnlockedData returns the locked data. Only available when account is unlocked.
func (sa *StoredAccount) GetUnlockedData() []byte {
	return sa.Data
}

// GetUnlockedData returns the locked data. Only available when account is unlocked.
func (sa *StoredAccount) GetCreatedAt() time.Time {
	return sa.CreatedAt
}

// Unlock decrypts the account using the given passphrase.
// It populates the decrypted cipher and private key fields.
func (sa *StoredAccount) Unlock(passphrase string) error {

	passphraseBs := hardenPassword([]byte(passphrase))
	acctBytes, err := util.Decrypt(sa.Cipher, passphraseBs[:])
	if err != nil {
		if funk.Contains(err.Error(), "invalid key") {
			return types.ErrInvalidPassprase
		}
		return err
	}
	sa.Data = acctBytes

	// Decode from base58
	acctData, _, err := base58.CheckDecode(string(acctBytes))
	if err != nil {
		return types.ErrInvalidPassprase
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

// AccountExist checks if an account with a matching address exists
func (am *AccountManager) AccountExist(address string) (bool, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return false, err
	}

	for _, acct := range accounts {
		if acct.GetAddress() == address {
			return true, nil
		}
	}

	return false, nil
}

// GetDefault gets the default account
func (am *AccountManager) GetDefault() (core.StoredAccount, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	for _, a := range accounts {
		if a.IsDefault() {
			return a, nil
		}
	}

	return nil, types.ErrAccountUnknown
}

// GetByIndex returns an account by its current position in the
// list of accounts which is ordered by the time of creation.
func (am *AccountManager) GetByIndex(i int) (core.StoredAccount, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	if acctLen := len(accounts); acctLen-1 < i {
		return nil, types.ErrAccountUnknown
	}

	return accounts[i], nil
}

// GetByIndexOrAddress gets an account by either its address or index
func (am *AccountManager) GetByIndexOrAddress(idxOrAddr string) (core.StoredAccount, error) {
	if crypto.IsValidAddr(idxOrAddr) == nil {
		return am.GetByAddress(idxOrAddr)
	}
	if govalidator.IsNumeric(idxOrAddr) {
		idx, _ := strconv.Atoi(idxOrAddr)
		return am.GetByIndex(idx)
	}
	return nil, types.ErrAccountUnknown
}

// GetByAddress gets an account by its address in the list of accounts.
func (am *AccountManager) GetByAddress(addr string) (core.StoredAccount, error) {

	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	account := funk.Find(accounts, func(x core.StoredAccount) bool {
		return x.GetAddress() == addr
	})

	if account == nil {
		return nil, types.ErrAccountUnknown
	}

	return account.(*StoredAccount), nil
}
