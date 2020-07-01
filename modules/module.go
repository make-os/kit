package modules

import (
	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/dht/server/types"
	"gitlab.com/makeos/mosdef/extensions"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/mempool"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/rpc"
	types2 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
)

// Module implements ModulesHub. It is a hub for other modules.
type Module struct {
	cfg            *config.AppConfig
	DefaultModules *modulestypes.Modules
	modulesCreator func() *modulestypes.Modules
}

// New creates an instance of Module
func New(
	cfg *config.AppConfig,
	acctmgr *keystore.Keystore,
	service services.Service,
	logic core.Logic,
	mempoolReactor *mempool.Reactor,
	ticketmgr types2.TicketManager,
	dht types.DHT,
	extMgr *extensions.Manager,
	rpcServer *rpc.Server,
	repoMgr core.RemoteServer) *Module {

	newModules := func() *modulestypes.Modules {
		return &modulestypes.Modules{
			Tx:      NewTxModule(service, logic),
			Chain:   NewChainModule(service, logic),
			Account: NewUserModule(cfg, acctmgr, service, logic),
			PushKey: NewPushKeyModule(cfg, service, logic),
			Ticket:  NewTicketModule(service, logic, ticketmgr),
			Repo:    NewRepoModule(service, repoMgr, logic),
			NS:      NewNSModule(service, repoMgr, logic),
			DHT:     NewDHTModule(cfg, dht),
			ExtMgr:  extMgr,
			Util:    NewUtilModule(),
			RPC:     NewRPCModule(cfg, rpcServer),
			Pool:    NewPoolModule(mempoolReactor, repoMgr.GetPushPool()),
		}
	}

	return &Module{
		cfg:            cfg,
		modulesCreator: newModules,
		DefaultModules: newModules(),
	}
}

// CreateNewModules creates and returns a new Modules instance
func (m *Module) CreateNewModules() *modulestypes.Modules {
	return m.modulesCreator()
}

// GetModules returns all sub-modules
func (m *Module) GetModules() *modulestypes.Modules {
	return m.DefaultModules
}

// ConfigureVM instructs VM-accessible modules accessible to configure the VM
func (m *Module) ConfigureVM(vm *otto.Otto) (sugs []prompt.Suggest) {
	return m.DefaultModules.ConfigureVM(vm, m.cfg.ConsoleOnly())
}
