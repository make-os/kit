package node

import (
	"context"
	"fmt"
	"os"
	"sync"

	dhtserver "github.com/make-os/lobe/dht/server"
	"github.com/make-os/lobe/dht/types"
	types2 "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/remote/server"
	rpcApi "github.com/make-os/lobe/rpc/api"
	storagetypes "github.com/make-os/lobe/storage/types"
	tickettypes "github.com/make-os/lobe/ticket/types"
	"github.com/make-os/lobe/types/core"

	"github.com/make-os/lobe/modules"
	"github.com/make-os/lobe/util"
	"github.com/thoas/go-funk"

	"github.com/make-os/lobe/extensions"
	"github.com/make-os/lobe/keystore"
	"github.com/robertkrimen/otto"
	"github.com/tendermint/tendermint/node"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/make-os/lobe/ticket"

	"github.com/make-os/lobe/mempool"

	"github.com/make-os/lobe/logic"

	"github.com/make-os/lobe/node/services"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"

	"github.com/make-os/lobe/storage"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/pkgs/logger"
	tmconfig "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	nm "github.com/tendermint/tendermint/node"
)

// RPCServer represents the client
type Node struct {
	app            *App
	cfg            *config.AppConfig
	acctMgr        *keystore.Keystore
	nodeKey        *p2p.NodeKey
	log            logger.Logger
	db             storagetypes.Engine
	stateTreeDB    storagetypes.Engine
	tm             *nm.Node
	service        services.Service
	logic          core.AtomicLogic
	mempoolReactor *mempool.Reactor
	ticketMgr      tickettypes.TicketManager
	dht            types.DHT
	modules        types2.ModulesHub
	remoteServer   core.RemoteServer
	closeOnce      *sync.Once
}

// NewNode creates an instance of RPCServer
func NewNode(cfg *config.AppConfig) *Node {
	return &Node{
		cfg:       cfg,
		nodeKey:   cfg.G().NodeKey,
		log:       cfg.G().Log.Module("node"),
		service:   services.New(cfg.G().TMConfig.RPC.ListenAddress),
		acctMgr:   keystore.New(cfg.KeystoreDir()),
		closeOnce: &sync.Once{},
	}
}

// OpenDB opens the database. In dev mode, create a
// namespace and open database file prefixed with
// the node ID as namespace
func (n *Node) OpenDB() error {

	if n.db != nil {
		return fmt.Errorf("db already open")
	}

	db := storage.NewBadger()
	if err := db.Init(n.cfg.GetAppDBDir()); err != nil {
		return err
	}

	stateTreeDB := storage.NewBadger()
	if err := stateTreeDB.Init(n.cfg.GetStateTreeDBDir()); err != nil {
		return err
	}

	n.db = db
	n.stateTreeDB = stateTreeDB
	return nil
}

// createCustomMempool creates a custom mempool and mempool reactor
// to replace tendermint's default mempool
func createCustomMempool(cfg *config.AppConfig, logic core.Logic) *nm.CustomMempool {
	memp := mempool.NewMempool(cfg, logic)
	mempReactor := mempool.NewReactor(cfg, memp)
	return &nm.CustomMempool{
		Mempool:        memp,
		MempoolReactor: mempReactor,
	}
}

// Start starts the tendermint node
func (n *Node) Start() error {

	if n.cfg.IsAttachMode() {
		return n.startConsoleOnly()
	}

	n.log.Info("Starting node...", "NodeID", n.cfg.G().NodeKey.ID(), "DevMode", n.cfg.IsDev())

	tmLog := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	var err error
	tmLog, err = tmflags.ParseLogLevel(n.cfg.G().TMConfig.LogLevel, tmLog, tmconfig.DefaultLogLevel())
	if err != nil {
		return errors.Wrap(err, "failed to parse log level")
	}

	if err := n.OpenDB(); err != nil {
		n.log.Fatal("Failed to open database", "Err", err)
	}

	n.log.Info("App database has been loaded", "AppDBDir", n.cfg.GetAppDBDir())

	// Read private validator
	pv := privval.LoadFilePV(n.cfg.G().TMConfig.PrivValidatorKeyFile(), n.cfg.G().TMConfig.PrivValidatorStateFile())

	// Create an atomic logic provider
	n.logic = logic.NewAtomic(n.db, n.stateTreeDB, n.cfg)

	// Create ticket manager
	n.ticketMgr = ticket.NewManager(n.logic.GetDBTx(), n.cfg, n.logic)
	n.logic.SetTicketManager(n.ticketMgr)

	// Initialize and start the DHT module (only in non-validator mode)
	if !n.cfg.IsValidatorNode() {
		n.dht, err = dhtserver.New(context.Background(), n.logic, n.cfg)
		if err != nil {
			return err
		}
		if err = n.dht.Start(); err != nil {
			return err
		}
	}

	// Create the ABCI app and wrap with a ClientCreator
	app := NewApp(n.cfg, n.db, n.logic, n.ticketMgr)
	clientCreator := proxy.NewLocalClientCreator(app)

	// Create custom mempool and set the epoch seed generator function
	cusMemp := createCustomMempool(n.cfg, n.logic)
	memp := cusMemp.Mempool.(*mempool.Mempool)
	mempR := cusMemp.MempoolReactor.(*mempool.Reactor)

	// Pass mempool reactor to logic
	n.logic.SetMempoolReactor(mempR)

	// Create remote server and pass it to logic
	remoteServer := server.New(n.cfg, n.cfg.Remote.Address, n.logic, n.dht, memp, n)
	n.remoteServer = remoteServer
	n.logic.SetRemoteServer(remoteServer)
	for _, ch := range remoteServer.GetChannels() {
		node.AddChannels([]byte{ch.ID})
	}

	// Create node
	n.tm, err = nm.NewNodeWithCustomMempool(
		n.cfg.G().TMConfig,
		pv,
		n.nodeKey,
		clientCreator,
		cusMemp,
		nm.DefaultGenesisDocProviderFunc(n.cfg.G().TMConfig),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(n.cfg.G().TMConfig.Instrumentation),
		tmLog)
	if err != nil {
		return errors.Wrap(err, "failed to fully create node")
	}

	// Register the custom reactors
	n.tm.Switch().AddReactor("PushReactor", remoteServer)

	// Pass the proxy app to the mempool
	memp.SetProxyApp(n.tm.ProxyApp().Mempool())

	fullAddr := fmt.Sprintf("%s@%s", n.nodeKey.ID(), n.cfg.G().TMConfig.P2P.ListenAddress)
	n.log.Info("Now listening for connections", "Address", fullAddr)

	// Set references of various instances on the node
	n.mempoolReactor = mempR

	// Pass repo manager to logic manager
	n.logic.SetRemoteServer(remoteServer)

	// Start tendermint
	if err := n.tm.Start(); err != nil {
		n.Stop()
		return err
	}

	// Initialize extension manager and start extensions
	n.configureInterfaces()

	return nil
}

// startConsoleOnly configures modules, extension manager and the RPC server.
// However, the RPC server is not started.
func (n *Node) startConsoleOnly() error {

	// Initialize and start JS modules and extensions
	n.configureInterfaces()

	return nil
}

// configureInterfaces configures:
// - Extension manager and runs extensions
// - Creates module aggregator
// - Registers methods to JSON-RPC 2.0 server
// - Initializes JS virtual machine context
func (n *Node) configureInterfaces() {

	vm := otto.New()

	// Create extension manager
	extMgr := extensions.NewManager(n.cfg)

	// Create module hub
	n.modules = modules.New(n.cfg, n.acctMgr, n.service, n.logic, n.mempoolReactor, n.ticketMgr, n.dht, extMgr, n.remoteServer)

	// Register JSON RPC methods
	if n.remoteServer != nil {
		n.remoteServer.GetRPCHandler().MergeAPISet(rpcApi.APIs(n.modules))
	}

	// Set the js module to be the main module of the extension manager
	extMgr.SetMainModule(n.modules)

	// ConfigureVM the js module if we are not in console-only mode
	if !n.isConsoleMode() {
		n.modules.ConfigureVM(vm)
	}

	// Parse the arguments and run extensions
	args, common := util.ParseExtArgs(n.cfg.Node.ExtensionsArgs)
	for _, name := range funk.UniqString(n.cfg.Node.Extensions) {
		args, ok := args[name]
		if !ok {
			args = common
		} else if ok {
			for k, v := range common {
				if _, ok := args[k]; !ok {
					args[k] = v
				}
			}
		}
		extMgr.Run(name, args)
	}
}

// GetBlock returns a tendermint block with the given height.
func (n *Node) GetBlock(height int64) *tmtypes.Block {
	return n.tm.BlockStore().LoadBlock(height)
}

// GetChainHeight returns the current chain height
func (n *Node) GetChainHeight() int64 {
	return n.tm.BlockStore().Height()
}

// isConsoleMode checks whether the console is running
func (n *Node) isConsoleMode() bool {
	return os.Args[1] == "console"
}

// GetModulesHub returns the modules hub
func (n *Node) GetModulesHub() types2.ModulesHub {
	return n.modules
}

// Stop the node
func (n *Node) Stop() {
	n.closeOnce.Do(func() {
		n.log.Info("Stopping...")

		if n.dht != nil {
			n.dht.Stop()
		}

		if n.tm != nil && n.tm.IsRunning() {
			n.tm.Stop()
		}

		if n.db != nil {
			n.db.Close()
		}

		if n.stateTreeDB != nil {
			n.stateTreeDB.Close()
		}

		if !n.cfg.IsAttachMode() {
			n.log.Info("Databases have been closed")
		}

		n.log.Info("Stopped")
	})
}
