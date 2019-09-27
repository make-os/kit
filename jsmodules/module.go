package jsmodules

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/accountmgr"
	"github.com/makeos/mosdef/mempool"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// Module provides functionalities that are accessible
// through the javascript console environment
type Module struct {
	nodeService types.Service
	logic       types.Logic
	txReactor   *mempool.Reactor
	acctmgr     *accountmgr.AccountManager
}

// NewModule creates an instance of Module
func NewModule(acctmgr *accountmgr.AccountManager,
	nodeService types.Service,
	logic types.Logic,
	txReactor *mempool.Reactor) *Module {
	return &Module{
		acctmgr:     acctmgr,
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
	sugs = append(sugs, NewAccountModule(vm, m.acctmgr, m.nodeService).Configure()...)
	return sugs
}
