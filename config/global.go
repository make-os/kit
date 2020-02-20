package config

import (
	golog "log"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"

	"github.com/tendermint/tendermint/privval"

	tmcfg "github.com/tendermint/tendermint/config"

	"github.com/olebedev/emitter"
	"github.com/tendermint/tendermint/p2p"
	"gitlab.com/makeos/mosdef/pkgs/logger"
)

// Globals holds references to global objects
type Globals struct {
	Log       logger.Logger
	Bus       *emitter.Emitter
	NodeKey   *p2p.NodeKey
	TMConfig  *tmcfg.Config
	PrivVal   *crypto.WrappedPV
	Interrupt *util.Interrupt
}

// G returns the global object
func (c *AppConfig) G() *Globals {
	return c.g
}

// PrepareNodeValKeys gets the node key from the node key file
// and caches it for fast access
func (c *AppConfig) PrepareNodeValKeys(
	nodeKeyFile,
	privValKeyFile,
	privValStateFile string) *p2p.NodeKey {

	// Load the node key
	nodeKey, err := p2p.LoadNodeKey(nodeKeyFile)
	if err != nil {
		golog.Fatalf("Failed to load node key: " + err.Error())
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
