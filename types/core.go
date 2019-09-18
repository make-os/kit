package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/util"
	"github.com/robertkrimen/otto"
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
	Configure(vm *otto.Otto) []prompt.Suggest
}

// Service provides an interface for exposing functionalities.
// It is meant to be used by packages that offer operations
// that other packages or processes might need
type Service interface {
	Do(method string, param interface{}) (interface{}, error)
}

// Account represents a user's identity and includes
// balance and other information.
type Account struct {
	Balance util.String
	Nonce   int64
}

// Bytes return the bytes equivalent of the account
func (a *Account) Bytes() []byte {
	return util.ObjectToBytes(a)
}
