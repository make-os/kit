package block

import (
	"github.com/makeos/mosdef/types"
)

// Block manages the block state, provides access
// to block records and other associated information.
type Block struct {
	Keepers types.Keepers
}
