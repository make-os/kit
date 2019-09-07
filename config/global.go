package config

import (
	"github.com/ellcrys/elld/elldb"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util/logger"
	"github.com/olebedev/emitter"
)

// Globals holds references to global objects
type Globals struct {
	DB      elldb.DB
	Log     logger.Logger
	Bus     *emitter.Emitter
	NodeKey *crypto.Key
}

// G returns the global object
func (c *EngineConfig) G() *Globals {
	return c.g
}
