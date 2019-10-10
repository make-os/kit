package types

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

// CommonNode describes the properties of a node
// that exposes common functionalities of a local
// node
type CommonNode interface {
	// GetCurrentValidators returns the current validators
	GetCurrentValidators() []*tmtypes.Validator
}
