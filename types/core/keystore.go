package core

import (
	"time"

	"github.com/c-bata/go-prompt"
	"gitlab.com/makeos/mosdef/crypto"
)

type KeyType int

const (
	KeyTypeAccount KeyType = iota
	KeyTypePush
)

// StoredKey represents a locally persisted key
type StoredKey interface {
	GetMeta() StoredKeyMeta
	GetKey() *crypto.Key
	GetPayload() *KeyPayload
	Unlock(passphrase string) error
	GetFilename() string
	GetAddress() string
	IsUnprotected() bool
	GetType() KeyType
	GetUnlockedData() []byte
	GetCreatedAt() time.Time
}

type AccountManager interface {
	Configure() []prompt.Suggest
	UpdateCmd(addressOrIndex, passphrase string) error
	RevealCmd(addrOrIdx, pass string) error
	ListAccounts() (accounts []StoredKey, err error)
	ListCmd() error
	CreateAccount(defaultAccount bool, address *crypto.Key, passphrase string) error
	CreateCmd(defaultAccount bool, seed int64, pass string) (*crypto.Key, error)
	ImportCmd(keyFile, pass string) error
	AskForPassword() (string, error)
	AskForPasswordOnce() string
	AccountExist(address string) (bool, error)
	GetDefault() (StoredKey, error)
	GetByIndex(i int) (StoredKey, error)
	GetByIndexOrAddress(idxOrAddr string) (StoredKey, error)
	GetByAddress(addr string) (StoredKey, error)
	UIUnlockAccount(addressOrIndex, passphrase string) (StoredKey, error)
}

// StoredKeyMeta represents additional meta data of an account
type StoredKeyMeta map[string]interface{}

// HasKey checks whether a key exist
func (sm StoredKeyMeta) HasKey(key string) bool {
	_, ok := sm[key]
	return ok
}

// Get returns a value
func (sm StoredKeyMeta) Get(key string) interface{} {
	return sm[key]
}

// KeyPayload contains key data that will  be stored on disk
type KeyPayload struct {
	SecretKey     string `json:"secretKey" msgpack:"secretKey"`
	Type          int    `json:"type" msgpack:"type"`
	FormatVersion string `json:"version" msgpack:"version"`
}
