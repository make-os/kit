package jsmodules

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/accountmgr"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/mempool"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// Module provides functionalities that are accessible
// through the javascript console environment
type Module struct {
	cfg       *config.EngineConfig
	service   types.Service
	logic     types.Logic
	txReactor *mempool.Reactor
	acctmgr   *accountmgr.AccountManager
	ticketmgr types.TicketManager
}

// NewModule creates an instance of Module
func NewModule(
	cfg *config.EngineConfig,
	acctmgr *accountmgr.AccountManager,
	service types.Service,
	logic types.Logic,
	txReactor *mempool.Reactor,
	ticketmgr types.TicketManager) *Module {
	return &Module{
		cfg:       cfg,
		acctmgr:   acctmgr,
		service:   service,
		logic:     logic,
		txReactor: txReactor,
		ticketmgr: ticketmgr,
	}
}

// Configure initialized the module and all sub-modules
func (m *Module) Configure(vm *otto.Otto) []prompt.Suggest {
	nodeSrv := m.service
	sugs := []prompt.Suggest{}
	sugs = append(sugs, NewTxModule(vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewChainModule(vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewPoolModule(vm, m.txReactor).Configure()...)
	sugs = append(sugs, NewAccountModule(m.cfg, vm, m.acctmgr, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewGPGModule(m.cfg, vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewUtilModule(vm).Configure()...)
	sugs = append(sugs, NewTicketModule(vm, nodeSrv, m.ticketmgr).Configure()...)
	sugs = append(sugs, NewRepoModule(vm, nodeSrv, m.logic).Configure()...)
	return sugs
}
