package rest

import "gitlab.com/makeos/mosdef/types"

// Rest provides a REST API handlers
type Rest struct {
	mods types.ModulesAggregator
}

// New creates an instance of Rest
func New(mods types.ModulesAggregator) *Rest {
	return &Rest{mods}
}
