package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// ModulesAggregatorFunc describes a JS module function
type ModulesAggregatorFunc struct {
	Name        string
	Value       interface{}
	Description string
}

// ModulesAggregator describes a mechanism for aggregating, configuring and
// accessing modules that provide uniform functionalities in JS environment,
// JSON-RPC APIs and REST APIs
type ModulesAggregator interface {
	ConfigureVM(vm *otto.Otto) []prompt.Suggest
	GetModules() interface{}
}

