package modules

import (
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/lobe/api/rpc/client"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/dht/server/types"
	"gitlab.com/makeos/lobe/extensions"
	"gitlab.com/makeos/lobe/keystore"
	"gitlab.com/makeos/lobe/mempool"
	modulestypes "gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/node/services"
	"gitlab.com/makeos/lobe/rpc"
	types2 "gitlab.com/makeos/lobe/ticket/types"
	"gitlab.com/makeos/lobe/types/core"
)

// Module implements ModulesHub. It is a hub for other modules.
type Module struct {
	cfg        *config.AppConfig
	attachMode bool
	Modules    *modulestypes.Modules
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

// NewAttachable creates an instance of Module configured for attach mode.
func NewAttachable(cfg *config.AppConfig, client client.Client, ks *keystore.Keystore) *Module {
	return &Module{
		cfg:        cfg,
		attachMode: cfg.IsAttachMode(),
		Modules: &modulestypes.Modules{
			Tx:      NewAttachableTxModule(client),
			Chain:   NewAttachableChainModule(client),
			User:    NewAttachableUserModule(client, ks),
			PushKey: NewAttachablePushKeyModule(client),
			Ticket:  NewAttachableTicketModule(client),
			Repo:    NewAttachableRepoModule(client),
			NS:      NewAttachableNamespaceModule(client),
			DHT:     NewAttachableDHTModule(client),
			Util:    NewConsoleUtilModule(os.Stdout),
			RPC:     NewRPCModule(cfg, nil),
			Pool:    NewAttachablePoolModule(client),
		},
	}
}

// GetModules returns all sub-modules
func (m *Module) GetModules() *modulestypes.Modules {
	return m.Modules
}

// ConfigureVM instructs VM-accessible modules accessible to configure the VM
func (m *Module) ConfigureVM(vm *otto.Otto) (sugs []prompt.Completer) {
	return m.Modules.ConfigureVM(vm)
}
