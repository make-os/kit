package modules

import (
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/extensions"
	"github.com/make-os/kit/keystore"
	"github.com/make-os/kit/mempool"
	"github.com/make-os/kit/miner"
	modulestypes "github.com/make-os/kit/modules/types"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/node/services"
	types3 "github.com/make-os/kit/rpc/types"
	types2 "github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/types/core"
	"github.com/robertkrimen/otto"
)

// Module implements ModulesHub. It is a hub for other modules.
type Module struct {
	cfg        *config.AppConfig
	attachMode bool
	Modules    *modulestypes.Modules
}

// New creates an instance of Module
func New(cfg *config.AppConfig, acctmgr *keystore.Keystore, service services.Service, logic core.Logic,
	mempoolReactor *mempool.Reactor, ticketmgr types2.TicketManager, dht dht2.DHT,
	extMgr *extensions.Manager, remoteSvr core.RemoteServer, miner miner.Miner) *Module {

	return &Module{
		cfg: cfg,
		Modules: &modulestypes.Modules{
			Tx:      NewTxModule(service, logic),
			Chain:   NewChainModule(service, logic),
			User:    NewUserModule(cfg, acctmgr, service, logic),
			PushKey: NewPushKeyModule(cfg, service, logic),
			Ticket:  NewTicketModule(service, logic, ticketmgr),
			Repo:    NewRepoModule(service, remoteSvr, logic),
			NS:      NewNamespaceModule(service, remoteSvr, logic),
			DHT:     NewDHTModule(cfg, dht),
			ExtMgr:  extMgr,
			Util:    NewConsoleUtilModule(os.Stdout),
			RPC:     NewRPCModule(cfg),
			Pool:    NewPoolModule(mempoolReactor, remoteSvr.GetPushPool()),
			Dev:     NewDevModule(),
			Miner:   NewMinerModule(cfg, miner),
		},
	}
}

// NewAttachable creates an instance of Module configured for attach mode.
func NewAttachable(cfg *config.AppConfig, client types3.Client, ks *keystore.Keystore) *Module {
	return &Module{
		cfg:        cfg,
		attachMode: cfg.IsAttachMode(),
		Modules: &modulestypes.Modules{
			Tx:      NewAttachableTxModule(client),
			Chain:   NewAttachableChainModule(client),
			User:    NewAttachableUserModule(cfg, client, ks),
			PushKey: NewAttachablePushKeyModule(cfg, client),
			Ticket:  NewAttachableTicketModule(client),
			Repo:    NewAttachableRepoModule(client),
			NS:      NewAttachableNamespaceModule(client),
			DHT:     NewAttachableDHTModule(cfg, client),
			Util:    NewConsoleUtilModule(os.Stdout),
			RPC:     NewRPCModule(cfg),
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
