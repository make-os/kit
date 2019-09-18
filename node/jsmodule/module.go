package jsmodule

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// Module provides functionalities that are accessible
// through the javascript console environment
type Module struct {
	nodeService types.Service
}

// NewModule creates an instance of Module for account management.
// Pass the node service so it can perform node specific operations.
func NewModule(nodeService types.Service) *Module {
	return &Module{
		nodeService: nodeService,
	}
}

// Configure initialized the module and all sub-modules
func (m *Module) Configure(vm *otto.Otto) []prompt.Suggest {
	sugs := []prompt.Suggest{}
	sugs = append([]prompt.Suggest{}, NewTxModule(vm, m.nodeService).Configure()...)
	sugs = append([]prompt.Suggest{}, NewBlockModule(vm, m.nodeService).Configure()...)
	return sugs
}
