package modules

import (
	"github.com/c-bata/go-prompt"
	"github.com/fatih/structs"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/dht/server/types"
	"gitlab.com/makeos/mosdef/extensions"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/rpc"
	types2 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
)

// Module is a hub for other modules.
type Module struct {
	cfg     *config.AppConfig
	Modules *modules.Modules
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

	m := &Module{
		cfg: cfg,
		Modules: &modules.Modules{
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
		},
	}

	return m
}

// GetModules returns all sub-modules
func (m *Module) GetModules() *modules.Modules {
	return m.Modules
}

// ConfigureVM instructs VM-accessible modules accessible to configure the VM
func (m *Module) ConfigureVM(vm *otto.Otto) (sugs []prompt.Suggest) {
	for _, f := range structs.Fields(m.Modules) {
		mod := f.Value().(modules.Module)
		if !m.cfg.ConsoleOnly() {
			sugs = append(sugs, mod.ConfigureVM(vm)...)
			continue
		}

		if mod.ConsoleOnlyMode() {
			sugs = append(sugs, mod.ConfigureVM(vm)...)
		}
	}
	return
}
