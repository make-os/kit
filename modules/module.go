package modules

import (
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/dht/server/types"
	"github.com/themakeos/lobe/extensions"
	"github.com/themakeos/lobe/keystore"
	"github.com/themakeos/lobe/mempool"
	modulestypes "github.com/themakeos/lobe/modules/types"
	"github.com/themakeos/lobe/node/services"
	"github.com/themakeos/lobe/rpc"
	types2 "github.com/themakeos/lobe/ticket/types"
	"github.com/themakeos/lobe/types/core"
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
			Dev:     NewDevModule(),
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
			Dev:     NewDevModule(),
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
