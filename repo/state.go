package repo

import (
	"github.com/makeos/mosdef/util"
)

// State describes the current state of repository
type State struct {
	Refs *ObjCol
}

// IsEmpty checks whether the state is empty
func (s *State) IsEmpty() bool {
	return s.Refs.Len() == 0
}

// Hash returns the 32-bytes hash of the state
func (s *State) Hash() util.Hash {
	bz := util.ObjectToBytes([]interface{}{
		s.Refs.Bytes(),
	})
	return util.BytesToHash(util.Blake2b256(bz))
}

// GetChanges returns information about changes that occurred from
// state s to state y.
func (s *State) GetChanges(y *State) *Changes {

	var refChange *ChangeResult

	if y == nil {
		return &Changes{References: emptyChangeResult()}
	}

	// Check the refs for changes
	if s.Refs != nil {
		refChange = getRefChanges(s.Refs, y.Refs)
	}

	return &Changes{
		References: refChange,
	}
}
