package config

import (
	golog "log"

	"github.com/ellcrys/elld/elldb"
	"github.com/makeos/mosdef/util/logger"
	"github.com/olebedev/emitter"
	"github.com/tendermint/tendermint/p2p"
)

// Globals holds references to global objects
type Globals struct {
	DB      elldb.DB
	Log     logger.Logger
	Bus     *emitter.Emitter
	NodeKey *p2p.NodeKey
}

// G returns the global object
func (c *EngineConfig) G() *Globals {
	return c.g
}

// PrepareNodeKey gets the node key from the node key file
// and caches it for fast access
func (c *EngineConfig) PrepareNodeKey(nodeKeyFile string) *p2p.NodeKey {
	nodeKey, err := p2p.LoadNodeKey(nodeKeyFile)
	if err != nil {
		golog.Fatalf("Failed to load node key")
	}

	c.g.NodeKey = nodeKey
	return nodeKey
}
