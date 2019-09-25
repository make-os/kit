package jsmodule

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/node/reactors"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// Module provides functionalities that are accessible
// through the javascript console environment
type Module struct {
	nodeService types.Service
	logic       types.Logic
	txReactor   *reactors.TxReactor
}

// NewModule creates an instance of Module
func NewModule(nodeService types.Service, logic types.Logic, txReactor *reactors.TxReactor) *Module {
	return &Module{
		nodeService: nodeService,
		logic:       logic,
		txReactor:   txReactor,
	}
}

// Configure initialized the module and all sub-modules
func (m *Module) Configure(vm *otto.Otto) []prompt.Suggest {
	sugs := []prompt.Suggest{}
	sugs = append(sugs, NewTxModule(vm, m.nodeService).Configure()...)
	sugs = append(sugs, NewChainModule(vm, m.nodeService, m.logic).Configure()...)
	sugs = append(sugs, NewPoolModule(vm, m.txReactor).Configure()...)
	return sugs
}
