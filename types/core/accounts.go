package core

import (
	"time"

	"github.com/c-bata/go-prompt"
	"gitlab.com/makeos/mosdef/crypto"
)

// StoredAccount represents a locally persisted account
type StoredAccount interface {
	GetMeta() StoredAccountMeta
	GetKey() *crypto.Key
	Unlock(passphrase string) error
	IsDefault() bool
	GetAddress() string
	GetUnlockedData() []byte
	GetCreatedAt() time.Time
}

type AccountManager interface {
	Configure() []prompt.Suggest
	UpdateCmd(addressOrIndex, passphrase string) error
	RevealCmd(addrOrIdx, pass string) error
	ListAccounts() (accounts []StoredAccount, err error)
	ListCmd() error
	CreateAccount(defaultAccount bool, address *crypto.Key, passphrase string) error
	CreateCmd(defaultAccount bool, seed int64, pass string) (*crypto.Key, error)
	ImportCmd(keyFile, pass string) error
	AskForPassword() (string, error)
	AskForPasswordOnce() string
	AccountExist(address string) (bool, error)
	GetDefault() (StoredAccount, error)
	GetByIndex(i int) (StoredAccount, error)
	GetByIndexOrAddress(idxOrAddr string) (StoredAccount, error)
	GetByAddress(addr string) (StoredAccount, error)
	UIUnlockAccount(addressOrIndex, passphrase string) (StoredAccount, error)
}

// StoredAccountMeta represents additional meta data of an account
type StoredAccountMeta map[string]interface{}

// HasKey checks whether a key exist
func (sm StoredAccountMeta) HasKey(key string) bool {
	_, ok := sm[key]
	return ok
}

// Get returns a value
func (sm StoredAccountMeta) Get(key string) interface{} {
	return sm[key]
}
