package node

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/makeos/mosdef/rpc"

	jsm "github.com/makeos/mosdef/jsmodules"
	"github.com/makeos/mosdef/util"
	"github.com/thoas/go-funk"

	"github.com/makeos/mosdef/accountmgr"
	"github.com/makeos/mosdef/dht"
	"github.com/makeos/mosdef/extensions"
	"github.com/makeos/mosdef/repo"
	"github.com/robertkrimen/otto"

	"github.com/tendermint/tendermint/node"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/makeos/mosdef/ticket"

	"github.com/makeos/mosdef/mempool"

	logic "github.com/makeos/mosdef/logic"

	"github.com/makeos/mosdef/tmrpc"

	"github.com/makeos/mosdef/services"

	"github.com/makeos/mosdef/types"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"

	"github.com/makeos/mosdef/storage"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/util/logger"
	tmconfig "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	nm "github.com/tendermint/tendermint/node"
)

// Node represents the client
type Node struct {
	app            *App
	cfg            *config.AppConfig
	acctMgr        *accountmgr.AccountManager
	tmcfg          *tmconfig.Config
	nodeKey        *p2p.NodeKey
	log            logger.Logger
	db             storage.Engine
	stateTreeDB    storage.Engine
	tm             *nm.Node
	service        types.Service
	tmrpc          *tmrpc.TMRPC
	logic          types.AtomicLogic
	mempoolReactor *mempool.Reactor
	ticketMgr      types.TicketManager
	dht            types.DHT
	jsModule       types.JSModule
	rpcServer      *rpc.Server
	repoMgr        types.RepoManager
}

// NewNode creates an instance of Node
func NewNode(cfg *config.AppConfig, tmcfg *tmconfig.Config) *Node {

	// Parse the RPC address
	parsedURL, err := url.Parse(tmcfg.RPC.ListenAddress)
	if err != nil {
		panic(errors.Wrap(err, "failed to parse RPC address"))
	}
	tmrpc := tmrpc.New(net.JoinHostPort(parsedURL.Hostname(), parsedURL.Port()))

	return &Node{
		cfg:     cfg,
		nodeKey: cfg.G().NodeKey,
		log:     cfg.G().Log.Module("node"),
		tmcfg:   tmcfg,
		service: services.New(tmrpc, nil, nil),
		tmrpc:   tmrpc,
		acctMgr: accountmgr.New(cfg.AccountDir()),
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

	if n.cfg.ConsoleOnly() {
		return n.startConsoleOnly()
	}

	n.log.Info("Starting node...", "NodeID", n.cfg.G().NodeKey.ID(), "DevMode", n.cfg.IsDev())

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	var err error
	logger, err = tmflags.ParseLogLevel(n.tmcfg.LogLevel, logger, tmconfig.DefaultLogLevel())
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
	if err != nil {
		return errors.Wrap(err, "failed to create ticket manager")
	}
	n.logic.SetTicketManager(n.ticketMgr)

	// Create DHT reactor and add it to the switch
	key, _ := n.cfg.G().PrivVal.GetKey()
	n.dht, err = dht.New(
		context.Background(),
		n.cfg, key.PrivKey().Key(),
		n.cfg.DHT.Address)
	if err != nil {
		return err
	}

	// Register custom reactor channels
	node.AddChannels([]byte{
		dht.DHTReactorChannel,
		repo.PushNoteReactorChannel,
		repo.PushOKReactorChannel,
	})

	// Create the ABCI app and wrap with a ClientCreator
	app := NewApp(n.cfg, n.db, n.logic, n.ticketMgr)
	clientCreator := proxy.NewLocalClientCreator(app)

	// Create custom mempool and set the epoch secret generator function
	cusMemp := createCustomMempool(n.cfg, n.log)
	memp := cusMemp.Mempool.(*mempool.Mempool)
	memp.SetEpochSecretGetter(n.logic.Sys().GetCurretEpochSecretTx)
	mempR := cusMemp.MempoolReactor.(*mempool.Reactor)

	// Create repository manager and pass it to logic
	repoMgr := repo.NewManager(n.cfg, n.cfg.RepoMan.Address, n.logic, n.dht, memp, n)
	n.repoMgr = repoMgr
	n.logic.SetRepoManager(repoMgr)

	// Create node
	tmNode, err := nm.NewNodeWithCustomMempool(
		n.tmcfg,
		pv,
		n.nodeKey,
		clientCreator,
		cusMemp,
		nm.DefaultGenesisDocProviderFunc(n.tmcfg),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(n.tmcfg.Instrumentation),
		logger)
	if err != nil {
		return errors.Wrap(err, "failed to fully create node")
	}

	// Add the custom reactors
	tmNode.Switch().AddReactor("DHT", n.dht.(*dht.DHT))
	tmNode.Switch().AddReactor("PushReactor", repoMgr)

	// Pass the proxy app to the mempool
	memp.SetProxyApp(tmNode.ProxyApp().Mempool())

	fullAddr := fmt.Sprintf("%s@%s", n.nodeKey.ID(), n.tmcfg.P2P.ListenAddress)
	n.log.Info("Node is now listening for connections", "Address", fullAddr)

	// Set references of various instances on the node
	n.tm = tmNode
	n.mempoolReactor = mempR
	n.service = services.New(n.tmrpc, n.logic, mempR)

	// Register some object finder on the dht
	n.dht.RegisterObjFinder(repo.RepoObjectModule, repoMgr)

	// Pass repo manager to logic manager
	n.logic.SetRepoManager(repoMgr)

	// Start tendermint
	if err := n.tm.Start(); err != nil {
		n.Stop()
		return err
	}

	// Start the RPC server
	n.startRPCServer()

	// Initialize extension manager and start extensions
	n.initJSModuleAndExtension()

	return nil
}

// startRPCServer starts RPC service
func (n *Node) startRPCServer() {
	if n.cfg.RPC.On {
		n.rpcServer = rpc.NewServer(n.cfg, n.log.Module("rpc-sever"), n.cfg.G().Interrupt)
		n.addRPCAPIs()
		go n.rpcServer.Serve()
	}
}

func (n *Node) addRPCAPIs() {
	n.rpcServer.AddAPI(n.acctMgr.APIs())
	if n.logic != nil {
		n.rpcServer.AddAPI(n.logic.(*logic.Logic).APIs())
	}
}

func (n *Node) startConsoleOnly() error {

	// Create the rpc server, add APIs but don't start it.
	// The console will need a non-nil instance to learn about the RPC methods.
	n.rpcServer = rpc.NewServer(n.cfg, n.log.Module("rpc-sever"), n.cfg.G().Interrupt)

	// Add RPC APIs
	n.addRPCAPIs()

	// Initialize and start JS modules and extensions
	n.initJSModuleAndExtension()

	return nil
}

// initJSModuleAndExtension initializes  and starts the extension manager
func (n *Node) initJSModuleAndExtension() {

	// Create extension manager
	vm := otto.New()
	extMgr := extensions.NewManager(n.cfg, vm)

	// Create the javascript module instance
	n.jsModule = jsm.NewModule(
		n.cfg,
		accountmgr.New(n.cfg.AccountDir()),
		n.service,
		n.logic,
		n.mempoolReactor,
		n.ticketMgr,
		n.dht,
		extMgr,
		n.rpcServer,
		n.repoMgr,
	)

	// Set the js module to be the main module of the extension manager
	extMgr.SetMainModule(n.jsModule)

	// Configure the js module if we are not in console mode
	if !n.ConsoleOn() {
		n.jsModule.ConfigureVM(vm)
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

// GetJSModule returns the javascript module instance
func (n *Node) GetJSModule() types.JSModule {
	return n.jsModule
}

// GetTicketManager returns the ticket manager
func (n *Node) GetTicketManager() types.TicketManager {
	return n.ticketMgr
}

// GetLogic returns the logic instance
func (n *Node) GetLogic() types.Logic {
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

// GetService returns the node's service
func (n *Node) GetService() types.Service {
	return n.service
}

// Stop the node
func (n *Node) Stop() {
	n.log.Info("mosdef is stopping...")

	if n.tm != nil && n.tm.IsRunning() {
		n.tm.Stop()
	}

	if n.db != nil {
		n.db.Close()
	}

	if n.stateTreeDB != nil {
		n.stateTreeDB.Close()
	}

	if n.rpcServer != nil {
		n.rpcServer.Stop()
	}

	if !n.cfg.ConsoleOnly() {
		n.log.Info("Databases have been closed")
	}

	n.log.Info("mosdef has stopped")
}
