package types

import (
	"io"
	"time"

	"github.com/themakeos/lobe/crypto"
)

type KeyType int

const (
	KeyTypeUser KeyType = iota
	KeyTypePush
)

// KeyTypeChar maps key type to their respective file prefix
var KeyTypeChar = map[KeyType]string{
	KeyTypeUser: "u",
	KeyTypePush: "p",
}

// StoredKey represents a locally persisted key
type StoredKey interface {
	GetMeta() StoredKeyMeta
	GetKey() *crypto.Key
	GetPayload() *KeyPayload
	Unlock(passphrase string) error
	GetFilename() string
	GetUserAddress() string
	GetPushKeyAddress() string
	IsUnprotected() bool
	GetType() KeyType
	GetUnlockedData() []byte
	GetCreatedAt() time.Time
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
	Type          int    `json:"type" msgpack:"type"`
	FormatVersion string `json:"version" msgpack:"version"`

	SecretKey string `json:"secretKey" msgpack:"secretKey"`
}

// Keystore describes a module for managing keys
type Keystore interface {
	SetOutput(out io.Writer)
	AskForPassword(prompt ...string) (string, error)
	AskForPasswordOnce(prompt ...string) string
	UnlockKeyUI(addressOrIndex, passphrase, promptMsg string) (StoredKey, string, error)
	UpdateCmd(addressOrIndex, passphrase string) error
	GetCmd(addrOrIdx, pass string, showPrivKey bool) error
	ImportCmd(keyfile string, keyType KeyType, pass string) error
	Exist(address string) (bool, error)
	GetByIndex(i int) (StoredKey, error)
	GetByIndexOrAddress(idxOrAddr string) (StoredKey, error)
	GetByAddress(addr string) (StoredKey, error)
	CreateKey(key *crypto.Key, keyType KeyType, passphrase string) error
	CreateCmd(keyType KeyType, seed int64, passphrase string, nopass bool) (*crypto.Key, error)
	List() (accounts []StoredKey, err error)
	ListCmd(out io.Writer) error
}
