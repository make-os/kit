package node

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"

	rpcApi "github.com/themakeos/lobe/api/rpc"
	dhtserver "github.com/themakeos/lobe/dht/server"
	"github.com/themakeos/lobe/dht/server/types"
	types2 "github.com/themakeos/lobe/modules/types"
	"github.com/themakeos/lobe/remote/server"
	"github.com/themakeos/lobe/rpc"
	tickettypes "github.com/themakeos/lobe/ticket/types"
	"github.com/themakeos/lobe/types/core"

	"github.com/themakeos/lobe/modules"
	"github.com/themakeos/lobe/util"
	"github.com/thoas/go-funk"

	"github.com/robertkrimen/otto"
	"github.com/tendermint/tendermint/node"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/themakeos/lobe/extensions"
	"github.com/themakeos/lobe/keystore"

	"github.com/themakeos/lobe/ticket"

	"github.com/themakeos/lobe/mempool"

	"github.com/themakeos/lobe/logic"

	"github.com/themakeos/lobe/node/services"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"

	"github.com/themakeos/lobe/storage"

	tmconfig "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	nm "github.com/tendermint/tendermint/node"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/pkgs/logger"
)

// RPCServer represents the client
type Node struct {
	app            *App
	cfg            *config.AppConfig
	acctMgr        *keystore.Keystore
	tmcfg          *tmconfig.Config
	nodeKey        *p2p.NodeKey
	log            logger.Logger
	db             storage.Engine
	stateTreeDB    storage.Engine
	tm             *nm.Node
	service        services.Service
	logic          core.AtomicLogic
	mempoolReactor *mempool.Reactor
	ticketMgr      tickettypes.TicketManager
	dht            types.DHT
	modules        types2.ModulesHub
	rpcServer      *rpc.RPCServer
	remoteServer   core.RemoteServer
}

// NewNode creates an instance of RPCServer
func NewNode(cfg *config.AppConfig, tmcfg *tmconfig.Config) *Node {

	// Parse tendermint RPC address
	tmRPCAddr, err := url.Parse(tmcfg.RPC.ListenAddress)
	if err != nil {
		panic(errors.Wrap(err, "failed to parse RPC address"))
	}

	return &Node{
		cfg:     cfg,
		nodeKey: cfg.G().NodeKey,
		log:     cfg.G().Log.Module("node"),
		tmcfg:   tmcfg,
		service: services.New(net.JoinHostPort(tmRPCAddr.Hostname(), tmRPCAddr.Port())),
		acctMgr: keystore.New(cfg.KeystoreDir()),
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
func createCustomMempool(cfg *config.AppConfig, log logger.Logger) *nm.CustomMempool {
	memp := mempool.NewMempool(cfg)
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
	tmLog, err = tmflags.ParseLogLevel(n.tmcfg.LogLevel, tmLog, tmconfig.DefaultLogLevel())
	if err != nil {
		return errors.Wrap(err, "failed to parse log level")
	}

	if err := n.OpenDB(); err != nil {
		n.log.Fatal("Failed to open database", "Err", err)
	}

	n.log.Info("App database has been loaded", "AppDBDir", n.cfg.GetAppDBDir())

	// Read private validator
	pv := privval.LoadFilePV(n.tmcfg.PrivValidatorKeyFile(), n.tmcfg.PrivValidatorStateFile())

	// Create an atomic logic provider
	n.logic = logic.NewAtomic(n.db, n.stateTreeDB, n.cfg)

	// Create ticket manager
	n.ticketMgr = ticket.NewManager(n.logic.GetDBTx(), n.cfg, n.logic)
	n.logic.SetTicketManager(n.ticketMgr)

	// Create DHT reactor and add it to the switch
	key, _ := n.cfg.G().PrivVal.GetKey()
	n.dht, err = dhtserver.New(context.Background(), n.cfg, key.PrivKey().Key(), n.cfg.DHT.Address)
	if err != nil {
		return err
	}
	if err = n.dht.Start(); err != nil {
		return err
	}

	// Create the ABCI app and wrap with a ClientCreator
	app := NewApp(n.cfg, n.db, n.logic, n.ticketMgr)
	clientCreator := proxy.NewLocalClientCreator(app)

	// Create custom mempool and set the epoch seed generator function
	cusMemp := createCustomMempool(n.cfg, n.log)
	memp := cusMemp.Mempool.(*mempool.Mempool)
	mempR := cusMemp.MempoolReactor.(*mempool.Reactor)

	// Pass mempool reactor to logic
	n.logic.SetMempoolReactor(mempR)

	// Create remote server and pass it to logic
	remoteServer := server.NewRemoteServer(n.cfg, n.cfg.Remote.Address, n.logic, n.dht, memp, n)
	n.remoteServer = remoteServer
	n.logic.SetRemoteServer(remoteServer)
	for _, ch := range remoteServer.GetChannels() {
		node.AddChannels([]byte{ch.ID})
	}

	// Create node
	n.tm, err = nm.NewNodeWithCustomMempool(
		n.tmcfg,
		pv,
		n.nodeKey,
		clientCreator,
		cusMemp,
		nm.DefaultGenesisDocProviderFunc(n.tmcfg),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(n.tmcfg.Instrumentation),
		tmLog)
	if err != nil {
		return errors.Wrap(err, "failed to fully create node")
	}

	// Register the custom reactors
	n.tm.Switch().AddReactor("PushReactor", remoteServer)

	// Pass the proxy app to the mempool
	memp.SetProxyApp(n.tm.ProxyApp().Mempool())

	fullAddr := fmt.Sprintf("%s@%s", n.nodeKey.ID(), n.tmcfg.P2P.ListenAddress)
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

	// Start the RPC server
	n.startRPCServer()

	// Initialize extension manager and start extensions
	n.configureInterfaces()

	// Pass the module aggregator to the repo manager
	n.remoteServer.RegisterAPIHandlers(n.modules)

	return nil
}

// startRPCServer starts RPC service
func (n *Node) startRPCServer() {
	if n.cfg.RPC.On {
		n.rpcServer = rpc.NewServer(n.cfg, n.log.Module("rpc-sever"), n.cfg.G().Interrupt)
		go n.rpcServer.Serve()
	}
}

// startConsoleOnly configures modules, extension manager and the RPC server.
// However, the RPC server is not started.
func (n *Node) startConsoleOnly() error {

	// Create the rpc server, add APIs but don't start it.
	// The console will need a non-nil instance to learn about the RPC methods.
	n.rpcServer = rpc.NewServer(n.cfg, n.log.Module("rpc-sever"), n.cfg.G().Interrupt)

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
	n.modules = modules.New(
		n.cfg,
		n.acctMgr,
		n.service,
		n.logic,
		n.mempoolReactor,
		n.ticketMgr,
		n.dht,
		extMgr,
		n.rpcServer,
		n.remoteServer,
	)

	// Register JSON RPC methods
	if n.rpcServer != nil {
		n.rpcServer.AddAPI(rpcApi.APIs(n.modules, n.rpcServer))
	}

	// Set the js module to be the main module of the extension manager
	extMgr.SetMainModule(n.modules)

	// ConfigureVM the js module if we are not in console-only mode
	if !n.ConsoleOn() {
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

// GetDB returns the database instance
func (n *Node) GetDB() storage.Engine {
	return n.db
}

// ConsoleOn checks whether the console is running
func (n *Node) ConsoleOn() bool {
	return os.Args[1] == "console"
}

// GetModulesHub returns the modules hub
func (n *Node) GetModulesHub() types2.ModulesHub {
	return n.modules
}

// GetTicketManager returns the ticket manager
func (n *Node) GetTicketManager() tickettypes.TicketManager {
	return n.ticketMgr
}

// GetLogic returns the logic instance
func (n *Node) GetLogic() core.Logic {
	return n.logic
}

// GetDHT returns the DHT service
func (n *Node) GetDHT() types.DHT {
	return n.dht
}

// GetMempoolReactor returns the mempool reactor
func (n *Node) GetMempoolReactor() *mempool.Reactor {
	return n.mempoolReactor
}

// GetCurrentValidators returns the current validators
func (n *Node) GetCurrentValidators() []*tmtypes.Validator {
	_, cv := n.tm.ConsensusState().GetValidators()
	return cv
}

// getService returns the node's service
func (n *Node) GetService() services.Service {
	return n.service
}

// Stop the node
func (n *Node) Stop() {
	n.log.Info("lobe is stopping...")

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

	n.log.Info("lobe has stopped")
}
