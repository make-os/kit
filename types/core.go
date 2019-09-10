package types

import (
	"github.com/c-bata/go-prompt"
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
// than other packages or processes might need
type Service interface {
	Do(method string, params ...interface{}) (interface{}, error)
}
