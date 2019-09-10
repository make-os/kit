package node

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/types"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"

	"github.com/makeos/mosdef/mosdb"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/util/logger"
	tmconfig "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	nm "github.com/tendermint/tendermint/node"
)

// Node represents the client
type Node struct {
	app     *App
	cfg     *config.EngineConfig
	tmcfg   *tmconfig.Config
	nodeKey *p2p.NodeKey
	log     logger.Logger
	db      mosdb.DB
	tm      *nm.Node
	service types.Service
}

// NewNode creates an instance of Node
func NewNode(cfg *config.EngineConfig, tmcfg *tmconfig.Config) *Node {
	return &Node{
		cfg:     cfg,
		nodeKey: cfg.G().NodeKey,
		log:     cfg.G().Log.Module("Node"),
		tmcfg:   tmcfg,
		service: NewService(),
	}
}

// OpenDB opens the database. In dev mode, create a
// namespace and open database file prefixed with
// the node ID as namespace
func (n *Node) OpenDB() error {

	if n.db != nil {
		return fmt.Errorf("db already open")
	}

	db := mosdb.NewDB(n.log)
	if err := db.Open(n.cfg.GetDBDir()); err != nil {
		return err
	}

	n.db = db
	return nil
}

// DB returns the database instance
func (n *Node) DB() mosdb.DB {
	return n.db
}

// GetService returns the node's service
func (n *Node) GetService() types.Service {
	return n.service
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

	// Create node
	app := NewApp(n.db)
	node, err := nm.NewNode(
		n.tmcfg,
		pv,
		n.nodeKey,
		proxy.NewLocalClientCreator(app),
		nm.DefaultGenesisDocProviderFunc(n.tmcfg),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(n.tmcfg.Instrumentation),
		logger)
	if err != nil {
		return errors.Wrap(err, "failed to fully create node")
	}

	n.tm = node
	n.tm.Start()

	return nil
}

// Stop the node
func (n *Node) Stop() {
	n.log.Info("mosdef is stopping...")

	// Close database
	if n.db != nil {
		n.log.Info("Database is closing")
		n.db.Close()
		n.log.Info("Database has been closed")
	}

	n.tm.Stop()
	n.tm.Wait()

	n.log.Info("mosdef has stopped")
}
