package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/util"
	"github.com/robertkrimen/otto"
)

// ModulesAggregatorFunc describes a JS module function
type ModulesAggregatorFunc struct {
	Name        string
	Value       interface{}
	Description string
}

// ModulesAggregator describes a mechanism for aggregating, configuring and
// accesssing modules that provide uniform functionalities in JS environment,
// JSON-RPC APIs and REST APIs
type ModulesAggregator interface {
	ConfigureVM(vm *otto.Otto) []prompt.Suggest
	GetModules() interface{}
}

// Service provides an interface for exposing functionalities.
// It is meant to be used by packages that offer operations
// that other packages or processes might need
type Service interface {
	SendTx(tx BaseTx) (util.Bytes32, error)
	GetBlock(height int64) (map[string]interface{}, error)
	GetCurrentHeight() (int64, error)
	GetNonce(address util.String) (uint64, error)
}
