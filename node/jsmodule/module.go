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
	logic       types.Logic
}

// NewModule creates an instance of Module for account management.
// Pass the node service so it can perform node specific operations.
func NewModule(nodeService types.Service, logic types.Logic) *Module {
	return &Module{
		nodeService: nodeService,
		logic:       logic,
	}
}

// Configure initialized the module and all sub-modules
func (m *Module) Configure(vm *otto.Otto) []prompt.Suggest {
	sugs := []prompt.Suggest{}
	sugs = append(sugs, NewTxModule(vm, m.nodeService).Configure()...)
	sugs = append(sugs, NewChainModule(vm, m.nodeService, m.logic).Configure()...)
	return sugs
}
