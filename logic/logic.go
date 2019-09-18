package logic

import (
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/types"
)

// Logic is the central point for defining and accessing
// and modifying different type of state.
type Logic struct {
	cfg       *config.EngineConfig
	db        storage.Engine
	tx        types.TxLogic
	stateTree *tree.SafeTree
}

// New creates an instance of Logic
func New(db storage.Engine, tree *tree.SafeTree, cfg *config.EngineConfig) *Logic {

	// Create the hub and keepers. Pass the hub to the
	// keepers so they can use it to access shared resources
	// and operations.
	hub := &Logic{db: db, stateTree: tree}
	hub.tx = &Transaction{logic: hub}
	hub.cfg = cfg

	return hub
}

// Tx returns the transaction logic
func (h *Logic) Tx() types.TxLogic {
	return h.tx
}

// DB returns the hubs db reference
func (h *Logic) DB() storage.Engine {
	return h.db
}

// StateTree returns the state tree
func (h *Logic) StateTree() *tree.SafeTree {
	return h.stateTree
}
