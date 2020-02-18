package api

import "github.com/makeos/mosdef/types"

// Rest provides REST API for repo-related operations
type Rest struct {
	mods types.ModulesAggregator
}

// NewREST creates an instance of Rest
func NewREST(mods types.ModulesAggregator) *Rest {
	return &Rest{mods}
}
