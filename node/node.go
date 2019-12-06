package node

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/makeos/mosdef/dht"
	"github.com/makeos/mosdef/repo"

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
	app         *App
	cfg         *config.EngineConfig
	tmcfg       *tmconfig.Config
	nodeKey     *p2p.NodeKey
	log         logger.Logger
	db          storage.Engine
	stateTreeDB storage.Engine
	tm          *nm.Node
	service     types.Service
	tmrpc       *tmrpc.TMRPC
	logic       types.AtomicLogic
	txReactor   *mempool.Reactor
	ticketMgr   types.TicketManager
}

// NewNode creates an instance of Node
func NewNode(cfg *config.EngineConfig, tmcfg *tmconfig.Config) *Node {

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
func createCustomMempool(cfg *config.EngineConfig, log logger.Logger) *nm.CustomMempool {
	memp := mempool.NewMempool(cfg)
	mempReactor := mempool.NewReactor(cfg, memp)
	return &nm.CustomMempool{
		Mempool:        memp,
		MempoolReactor: mempReactor,
	}
}

// Start starts the tendermint node
func (n *Node) Start() error {

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	var err error
	logger, err = tmflags.ParseLogLevel(n.tmcfg.LogLevel, logger, tmconfig.DefaultLogLevel())
	if err != nil {
		return errors.Wrap(err, "failed to parse log level")
	}

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

	// Create repository manager and pass it to logic
	rm := repo.NewManager(n.cfg, n.cfg.RepoMan.Address, n.logic)
	n.logic.SetRepoManager(rm)

	// Create the DHT
	key, _ := n.cfg.G().PrivVal.GetKey()
	dht, err := dht.New(context.Background(), key.PrivKey().Wrapped(), n.cfg.DHT.Address)
	if err != nil {
		return err
	}
	_ = dht

	// Create the ABCI app and wrap with a ClientCreator
	app := NewApp(n.cfg, n.db, n.logic, n.ticketMgr)
	app.node = n
	clientCreator := proxy.NewLocalClientCreator(app)

	// Create custom mempool and set the epoch secret generator function
	memp := createCustomMempool(n.cfg, n.log)
	memp.Mempool.(*mempool.Mempool).SetEpochSecretGetter(n.logic.Sys().GetCurretEpochSecretTx)

	// Create node
	node, err := nm.NewNodeWithCustomMempool(
		n.tmcfg,
		pv,
		n.nodeKey,
		clientCreator,
		memp,
		nm.DefaultGenesisDocProviderFunc(n.tmcfg),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(n.tmcfg.Instrumentation),
		logger)
	if err != nil {
		return errors.Wrap(err, "failed to fully create node")
	}

	// Pass the proxy app to the mempool
	memp.Mempool.(*mempool.Mempool).SetProxyApp(node.ProxyApp().Mempool())

	fullAddr := fmt.Sprintf("%s@%s", n.nodeKey.ID(), n.tmcfg.P2P.ListenAddress)
	n.log.Info("Node is now listening for connections", "Address", fullAddr)

	// Cache a reference of tendermint node
	n.tm = node

	// Create reactors and pass to service
	txReactor := mempool.NewReactor(n.cfg, memp.Mempool.(*mempool.Mempool))
	txReactor.SetSwitch(node.Switch())
	n.txReactor = txReactor
	n.service = services.New(n.tmrpc, n.logic, txReactor)

	// Start repository server
	if err := rm.Start(); err != nil {
		n.Stop()
		return errors.Wrap(err, "failed to start repo manager")
	}

	// Pass repo manager to logic manager
	n.logic.SetRepoManager(rm)

	// Start tendermint
	if err := n.tm.Start(); err != nil {
		n.Stop()
		return err
	}

	return nil
}

// GetDB returns the database instance
func (n *Node) GetDB() storage.Engine {
	return n.db
}

// GetTicketManager returns the ticket manager
func (n *Node) GetTicketManager() types.TicketManager {
	return n.ticketMgr
}

// GetLogic returns the logic instance
func (n *Node) GetLogic() types.Logic {
	return n.logic
}

// GetTxReactor returns the transaction reactor
func (n *Node) GetTxReactor() *mempool.Reactor {
	return n.txReactor
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

	if n.tm.IsRunning() {
		n.tm.Stop()
	}

	if n.db != nil {
		n.db.Close()
	}

	if n.stateTreeDB != nil {
		n.stateTreeDB.Close()
	}

	n.log.Info("Databases have been closed")
	n.log.Info("mosdef has stopped")
}
