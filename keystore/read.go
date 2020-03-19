package keystore

import (
	"fmt"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/btcsuite/btcutil/base58"
	funk "github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// StoredKey represents an encrypted key stored on disk
type StoredKey struct {

	// Type indicates the key type
	Type core.KeyType

	// Address is the key's address
	Address string

	// Cipher includes the encryption data
	Cipher []byte

	// Data contains the decrypted data.
	// Only available after account is unlocked.
	Data []byte

	// privKey is the actual ed25519 key
	privKey *crypto.Key

	// key is the actual key content stored on disk
	key *core.KeyPayload

	// CreatedAt represents the time the key was created and stored on disk
	CreatedAt time.Time

	// Unprotected indicates that the key is encrypted with a default passphrase.
	// An unprotected key is equivalent to a key that has no passphrase.
	Unprotected bool

	// The filename of the key file
	Filename string

	// Store arbitrary, non-persistent information about the key
	meta core.StoredKeyMeta
}

// GetMeta returns the meta information of the key
func (sk *StoredKey) GetMeta() core.StoredKeyMeta {
	return sk.meta
}

// IsUnprotected checks whether the key is encrypted using the default passphrase
func (sk *StoredKey) IsUnprotected() bool {
	return sk.Unprotected
}

// GetFilename returns the filename of the key file
func (sk *StoredKey) GetFilename() string {
	return sk.Filename
}

// GetType returns the key type
func (sk *StoredKey) GetType() core.KeyType {
	return sk.Type
}

// GetAddress returns the address of the key
func (sk *StoredKey) GetAddress() string {
	return sk.Address
}

// GetKey returns the underlying Ed25519 key.
// Unlock() must be called first.
func (sk *StoredKey) GetKey() *crypto.Key {
	return sk.privKey
}

// GetKey returns the key object that is serialized and persisted.
func (sk *StoredKey) GetPayload() *core.KeyPayload {
	return sk.key
}

// GetUnlockedData returns the locked data.
// Only available when key is unlocked.
func (sk *StoredKey) GetUnlockedData() []byte {
	return sk.Data
}

// GetUnlockedData returns the locked data.
// Only available when key is unlocked.
func (sk *StoredKey) GetCreatedAt() time.Time {
	return sk.CreatedAt
}

// Unlock decrypts the key using the given passphrase.
func (sk *StoredKey) Unlock(passphrase string) error {

	passphraseBs := hardenPassword([]byte(passphrase))
	decData, err := util.Decrypt(sk.Cipher, passphraseBs[:])
	if err != nil {
		if funk.Contains(err.Error(), "invalid key") {
			return types.ErrInvalidPassprase
		}
		return err
	}
	sk.Data = decData

	// Decode from base58
	keyData, _, err := base58.CheckDecode(string(decData))
	if err != nil {
		return types.ErrInvalidPassprase
	}

	// Decode from msgpack
	var key core.KeyPayload
	if err := util.ToObject(keyData, &key); err != nil {
		return fmt.Errorf("unable to parse key payload")
	}
	sk.key = &key

	// Convert the secret key to PrivKey object
	privKey, err := crypto.PrivKeyFromBase58(key.SecretKey)
	if err != nil {
		return err
	}
	sk.privKey = crypto.NewKeyFromPrivKey(privKey)

	return nil
}

// Exist checks if a key that matches the given address exists
func (ks *Keystore) Exist(address string) (bool, error) {

	accounts, err := ks.List()
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

// GetByIndex returns a key by its current position in the
// list of accounts which is ordered by the time of creation.
func (ks *Keystore) GetByIndex(i int) (core.StoredKey, error) {

	accounts, err := ks.List()
	if err != nil {
		return nil, err
	}

	if acctLen := len(accounts); acctLen-1 < i {
		return nil, types.ErrKeyUnknown
	}

	return accounts[i], nil
}

// GetByIndexOrAddress gets a key by either its address or index
func (ks *Keystore) GetByIndexOrAddress(idxOrAddr string) (core.StoredKey, error) {
	if crypto.IsValidAccountAddr(idxOrAddr) == nil {
		return ks.GetByAddress(idxOrAddr)
	}
	if govalidator.IsNumeric(idxOrAddr) {
		idx, _ := strconv.Atoi(idxOrAddr)
		return ks.GetByIndex(idx)
	}
	return nil, types.ErrKeyUnknown
}

// GetByAddress gets a key by its address in the list of accounts.
func (ks *Keystore) GetByAddress(addr string) (core.StoredKey, error) {

	accounts, err := ks.List()
	if err != nil {
		return nil, err
	}

	account := funk.Find(accounts, func(x core.StoredKey) bool {
		return x.GetAddress() == addr
	})

	if account == nil {
		return nil, types.ErrKeyUnknown
	}

	return account.(*StoredKey), nil
}
