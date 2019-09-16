package keepers

import (
	"github.com/makeos/mosdef/keepers/block"
	"github.com/makeos/mosdef/keepers/system"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
)

// Keepers is the central point for accessing
// all forms of state keepers on the node
type Keepers struct {
	db     storage.Engine
	block  types.BlockKeeper
	system types.SystemKeeper
}

// New creates an instance of Keepers
func New(db storage.Engine) *Keepers {
	hub := &Keepers{db: db}
	hub.block = &block.Block{Keepers: hub}
	hub.system = &system.System{Keepers: hub}
	return hub
}

// GetBlockKeeper returns the block keeper
func (h *Keepers) GetBlockKeeper() types.BlockKeeper {
	return h.block
}

// GetSystemKeeper returns the system keeper
func (h *Keepers) GetSystemKeeper() types.SystemKeeper {
	return h.system
}

// GetDB returns the hubs db reference
func (h *Keepers) GetDB() storage.Engine {
	return h.db
}
