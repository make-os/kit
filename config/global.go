package config

import (
	golog "log"

	"github.com/make-os/kit/crypto/ed25519"

	"github.com/tendermint/tendermint/privval"

	tmcfg "github.com/tendermint/tendermint/config"

	"github.com/make-os/kit/pkgs/logger"
	"github.com/olebedev/emitter"
	"github.com/tendermint/tendermint/p2p"
)

// Globals holds references to global objects
type Globals struct {
	Log      logger.Logger
	Bus      *emitter.Emitter
	NodeKey  *p2p.NodeKey
	TMConfig *tmcfg.Config
	PrivVal  *ed25519.FilePV
}

// G returns the global object
func (c *AppConfig) G() *Globals {
	return c.g
}

// LoadKeys gets the node key from the node key file
// and caches it for fast access
func (c *AppConfig) LoadKeys(nodeKeyFile, privValKeyFile, privValStateFile string) *p2p.NodeKey {

	// Load the node key
	nodeKey, err := p2p.LoadNodeKey(nodeKeyFile)
	if err != nil {
		golog.Fatalf("Failed to load node key: " + err.Error())
	}

	// Load the private validator
	pv := privval.LoadFilePV(privValKeyFile, privValStateFile)

	// Set references for node key and priv val
	c.g.NodeKey = nodeKey
	c.g.PrivVal = &ed25519.FilePV{FilePV: pv}

	return nodeKey
}
