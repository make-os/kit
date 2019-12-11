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
	cfg       *config.AppConfig
	service   types.Service
	logic     types.Logic
	mempoolReactor *mempool.Reactor
	acctmgr   *accountmgr.AccountManager
	ticketmgr types.TicketManager
	dht       types.DHT
}

// NewModule creates an instance of Module
func NewModule(
	cfg *config.AppConfig,
	acctmgr *accountmgr.AccountManager,
	service types.Service,
	logic types.Logic,
	mempoolReactor *mempool.Reactor,
	ticketmgr types.TicketManager,
	dht types.DHT) *Module {
	return &Module{
		cfg:       cfg,
		acctmgr:   acctmgr,
		service:   service,
		logic:     logic,
		mempoolReactor: mempoolReactor,
		ticketmgr: ticketmgr,
		dht:       dht,
	}
}

// Configure initialized the module and all sub-modules
func (m *Module) Configure(vm *otto.Otto) []prompt.Suggest {
	nodeSrv := m.service
	sugs := []prompt.Suggest{}
	sugs = append(sugs, NewTxModule(vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewChainModule(vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewPoolModule(vm, m.mempoolReactor).Configure()...)
	sugs = append(sugs, NewAccountModule(m.cfg, vm, m.acctmgr, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewGPGModule(m.cfg, vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewUtilModule(vm).Configure()...)
	sugs = append(sugs, NewTicketModule(vm, nodeSrv, m.ticketmgr).Configure()...)
	sugs = append(sugs, NewRepoModule(vm, nodeSrv, m.logic).Configure()...)
	sugs = append(sugs, NewDHTModule(m.cfg, vm, m.dht).Configure()...)
	sugs = append(sugs, NewExtentionModule(m.cfg, vm, m).Configure()...)
	return sugs
}
