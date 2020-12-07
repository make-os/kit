package keystore

import (
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/btcsuite/btcutil/base58"
	"github.com/make-os/kit/crypto"
	types2 "github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	crypto2 "github.com/make-os/kit/util/crypto"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
)

// StoredKey represents an encrypted key stored on disk
type StoredKey struct {

	// Type indicates the key type
	Type types2.KeyType

	// Address is the user address encoding of the key
	UserAddress string

	// PushAddress is the push address encoding of the key
	PushAddress string

	// Cipher includes the encryption data
	Cipher []byte

	// Data contains the decrypted data.
	// Only available after account is unlocked.
	Data []byte

	// privKey is the actual ed25519 key
	privKey *crypto.Key

	// key is the actual key content stored on disk
	key *types2.KeyPayload

	// CreatedAt represents the time the key was created and stored on disk
	CreatedAt time.Time

	// Unprotected indicates that the key is encrypted with a default passphrase.
	// An unprotected key is equivalent to a key that has no passphrase.
	Unprotected bool

	// The filename of the key file
	Filename string

	// Store arbitrary, non-persistent information about the key
	meta types2.StoredKeyMeta
}

// GetMeta returns the meta information of the key
func (sk *StoredKey) GetMeta() types2.StoredKeyMeta {
	if sk.meta == nil {
		sk.meta = map[string]interface{}{}
	}
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
func (sk *StoredKey) GetType() types2.KeyType {
	return sk.Type
}

// GetUserAddress returns the address of the key
func (sk *StoredKey) GetUserAddress() string {
	return sk.UserAddress
}

// GetPushKeyAddress returns the push address of the key
func (sk *StoredKey) GetPushKeyAddress() string {
	return sk.PushAddress
}

// GetKey returns the underlying Ed25519 key.
// Unlock() must be called first.
func (sk *StoredKey) GetKey() *crypto.Key {
	return sk.privKey
}

// GetKey returns the key object that is serialized and persisted.
func (sk *StoredKey) GetPayload() *types2.KeyPayload {
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
	decData, err := crypto2.Decrypt(sk.Cipher, passphraseBs[:])
	if err != nil {
		if funk.Contains(err.Error(), "invalid key") {
			return types.ErrInvalidPassphrase
		}
		return err
	}
	sk.Data = decData

	// Decode from base58
	keyData, _, err := base58.CheckDecode(string(decData))
	if err != nil {
		return types.ErrInvalidPassphrase
	}

	// Decode from msgpack
	var key types2.KeyPayload
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
		if acct.GetUserAddress() == address {
			return true, nil
		}
	}

	return false, nil
}

// GetByIndex returns a key by its current position in the
// list of accounts which is ordered by the time of creation.
func (ks *Keystore) GetByIndex(i int) (types2.StoredKey, error) {

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
func (ks *Keystore) GetByIndexOrAddress(idxOrAddr string) (types2.StoredKey, error) {
	if idxOrAddr == "" {
		return nil, fmt.Errorf("index or address of key is required")
	}
	if crypto.IsValidUserAddr(idxOrAddr) == nil || crypto.IsValidPushAddr(idxOrAddr) == nil {
		return ks.GetByAddress(idxOrAddr)
	}
	if govalidator.IsNumeric(idxOrAddr) {
		return ks.GetByIndex(cast.ToInt(idxOrAddr))
	}
	return nil, types.ErrKeyUnknown
}

// GetByAddress finds a key where its user address or push key address matches addr
func (ks *Keystore) GetByAddress(addr string) (types2.StoredKey, error) {

	accounts, err := ks.List()
	if err != nil {
		return nil, err
	}

	account := funk.Find(accounts, func(x types2.StoredKey) bool {
		return x.GetUserAddress() == addr || x.GetPushKeyAddress() == addr
	})

	if account == nil {
		return nil, types.ErrKeyUnknown
	}

	return account.(*StoredKey), nil
}
