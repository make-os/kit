package modules

import (
	"os"

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
	cfg     *config.AppConfig
	Modules *modulestypes.Modules
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
	rpcServer *rpc.RPCServer,
	repoMgr core.RemoteServer) *Module {

	return &Module{
		cfg: cfg,
		Modules: &modulestypes.Modules{
			Tx:      NewTxModule(service, logic),
			Chain:   NewChainModule(service, logic),
			User:    NewUserModule(cfg, acctmgr, service, logic),
			PushKey: NewPushKeyModule(cfg, service, logic),
			Ticket:  NewTicketModule(service, logic, ticketmgr),
			Repo:    NewRepoModule(service, repoMgr, logic),
			NS:      NewNamespaceModule(service, repoMgr, logic),
			DHT:     NewDHTModule(cfg, dht),
			ExtMgr:  extMgr,
			Util:    NewConsoleUtilModule(os.Stdout),
			RPC:     NewRPCModule(cfg, rpcServer),
			Pool:    NewPoolModule(mempoolReactor, repoMgr.GetPushPool()),
		},
	}
}

// GetModules returns all sub-modules
func (m *Module) GetModules() *modulestypes.Modules {
	return m.Modules
}

// ConfigureVM instructs VM-accessible modules accessible to configure the VM
func (m *Module) ConfigureVM(vm *otto.Otto) (sugs []prompt.Completer) {
	return m.Modules.ConfigureVM(vm, m.cfg.IsAttachMode())
}
