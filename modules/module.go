package modules

import (
	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/account"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/extensions"
	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/rpc"
	types2 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
)

// Module consists of submodules optimized for accessing through Javascript
// environment and suitable for reuse in JSON-RPC and REST APIs.
type Module struct {
	cfg            *config.AppConfig
	service        services.Service
	logic          core.Logic
	mempoolReactor *mempool.Reactor
	acctmgr        *account.AccountManager
	ticketmgr      types2.TicketManager
	dht            types.DHTNode
	extMgr         modules.ExtManager
	rpcServer      *rpc.Server
	repoMgr        core.RepoManager
	Modules        *modules.Modules
}

// new creates an instance of Module which aggregates and
// provides functionality of configuring supported modules
func New(
	cfg *config.AppConfig,
	acctmgr *account.AccountManager,
	service services.Service,
	logic core.Logic,
	mempoolReactor *mempool.Reactor,
	ticketmgr types2.TicketManager,
	dht types.DHTNode,
	extMgr *extensions.Manager,
	rpcServer *rpc.Server,
	repoMgr core.RepoManager) *Module {

	agg := &Module{
		cfg:            cfg,
		acctmgr:        acctmgr,
		service:        service,
		logic:          logic,
		mempoolReactor: mempoolReactor,
		ticketmgr:      ticketmgr,
		dht:            dht,
		extMgr:         extMgr,
		rpcServer:      rpcServer,
		repoMgr:        repoMgr,
		Modules:        &modules.Modules{},
	}

	return agg
}

// GetModules returns all sub-modules
func (m *Module) GetModules() *modules.Modules {
	return m.Modules
}

func (m *Module) registerModules(vm *otto.Otto) {
	m.Modules.Tx = NewTxModule(vm, m.service, m.logic)
	m.Modules.Chain = NewChainModule(vm, m.service, m.logic)
	m.Modules.Account = NewAccountModule(m.cfg, vm, m.acctmgr, m.service, m.logic)
	m.Modules.GPG = NewGPGModule(m.cfg, vm, m.service, m.logic)
	m.Modules.Ticket = NewTicketModule(vm, m.service, m.logic, m.ticketmgr)
	m.Modules.Repo = NewRepoModule(vm, m.service, m.repoMgr, m.logic)
	m.Modules.NS = NewNSModule(vm, m.service, m.repoMgr, m.logic)
	m.Modules.DHT = NewDHTModule(m.cfg, vm, m.dht)
	m.Modules.ExtMgr = m.extMgr
	m.Modules.Util = NewUtilModule(vm)
	m.Modules.RPC = NewRPCModule(m.cfg, vm, m.rpcServer)

	if !m.cfg.ConsoleOnly() {
		m.Modules.Pool = NewPoolModule(vm, m.mempoolReactor, m.repoMgr.GetPushPool())
	}
}

// ConfigureVM initialized the module and all sub-modules
func (m *Module) ConfigureVM(vm *otto.Otto) (sugs []prompt.Suggest) {

	m.registerModules(vm)

	if m.cfg.ConsoleOnly() {
		goto console_only
	}

	sugs = append(sugs, m.Modules.Tx.Configure()...)
	sugs = append(sugs, m.Modules.Chain.Configure()...)
	sugs = append(sugs, m.Modules.Pool.Configure()...)
	sugs = append(sugs, m.Modules.Account.Configure()...)
	sugs = append(sugs, m.Modules.GPG.Configure()...)
	sugs = append(sugs, m.Modules.Ticket.Configure()...)
	sugs = append(sugs, m.Modules.Repo.Configure()...)
	sugs = append(sugs, m.Modules.NS.Configure()...)
	sugs = append(sugs, m.Modules.DHT.Configure()...)
	sugs = append(sugs, m.Modules.ExtMgr.SetVM(vm).Configure()...)

console_only:
	sugs = append(sugs, m.Modules.Util.Configure()...)
	sugs = append(sugs, m.Modules.RPC.Configure()...)

	return sugs
}
