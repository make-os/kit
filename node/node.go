package node

import (
	"fmt"

	"github.com/makeos/mosdef/mosdb"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util/logger"
)

// Node represents the client
type Node struct {
	app     *App
	cfg     *config.EngineConfig
	nodeKey *crypto.Key
	log     logger.Logger
	db      mosdb.DB
	address util.NodeAddr
}

// NewNode creates an instance of Node
func NewNode(cfg *config.EngineConfig) *Node {
	return &Node{
		cfg:     cfg,
		nodeKey: cfg.G().NodeKey,
		log:     cfg.G().Log.Module("Node"),
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

// Serve starts the node's server
func (n *Node) Serve() {
	
}
