package config

import (
	golog "log"

	"github.com/makeos/mosdef/crypto"

	"github.com/tendermint/tendermint/privval"

	tmcfg "github.com/tendermint/tendermint/config"

	"github.com/makeos/mosdef/util/logger"
	"github.com/olebedev/emitter"
	"github.com/tendermint/tendermint/p2p"
)

// Globals holds references to global objects
type Globals struct {
	Log      logger.Logger
	Bus      *emitter.Emitter
	NodeKey  *p2p.NodeKey
	TMConfig *tmcfg.Config
	PrivVal  *crypto.WrappedPV
}

// G returns the global object
func (c *EngineConfig) G() *Globals {
	return c.g
}

// PrepareNodeValKeys gets the node key from the node key file
// and caches it for fast access
func (c *EngineConfig) PrepareNodeValKeys(
	nodeKeyFile,
	privValKeyFile,
	privValStateFile string) *p2p.NodeKey {

	// Load the node key
	nodeKey, err := p2p.LoadNodeKey(nodeKeyFile)
	if err != nil {
		golog.Fatalf("Failed to load node key")
	}

	// Load the private validator
	pv := privval.LoadFilePV(privValKeyFile, privValStateFile)

	// Set references for node key and priv val
	c.g.NodeKey = nodeKey
	c.g.PrivVal = &crypto.WrappedPV{
		FilePV: pv,
	}

	return nodeKey
}
