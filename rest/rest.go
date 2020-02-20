package rest

import (
	modtypes "gitlab.com/makeos/mosdef/modules/types"
)

// Rest provides a REST API handlers
type Rest struct {
	mods modtypes.ModulesAggregator
}

// New creates an instance of Rest
func New(mods modtypes.ModulesAggregator) *Rest {
	return &Rest{mods}
}
