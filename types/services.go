package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/util"
	"github.com/robertkrimen/otto"
)

// Various names for staking categories
const (
	StakeTypeValidator = "v"
	StakeTypeStorer    = "s"
)

// JSModuleFunc describes a JS module function
type JSModuleFunc struct {
	Name        string
	Value       interface{}
	Description string
}

// JSModule describes a mechanism for providing functionalities
// accessible in the JS console environment.
type JSModule interface {
	ConfigureVM(vm *otto.Otto) []prompt.Suggest
}

// Service provides an interface for exposing functionalities.
// It is meant to be used by packages that offer operations
// that other packages or processes might need
type Service interface {
	SendTx(tx *Transaction) (util.Hash, error)
	GetBlock(height int64) (map[string]interface{}, error)
	GetCurrentHeight() (int64, error)
	GetNonce(address util.String) (uint64, error)
}
