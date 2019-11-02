package repo

import (
	"github.com/makeos/mosdef/util"
)

// State describes the current state of repository
type State struct {
	Tags *ObjCol
	Refs *ObjCol
}

// IsEmpty checks whether the state is empty
func (s *State) IsEmpty() bool {
	return s.Tags.Len() == 0 && s.Refs.Len() == 0
}

// Hash returns the 32-bytes hash of the state
func (s *State) Hash() util.Hash {
	bz := util.ObjectToBytes([]interface{}{
		s.Tags.Bytes(),
		s.Refs.Bytes(),
	})
	return util.BytesToHash(util.Blake2b256(bz))
}

// GetChanges returns information about changes that occurred from
// state s to state y.
func (s *State) GetChanges(y *State) *Changes {

	// Check the refs for changes
	var refChange *ChangeResult
	if s.Refs != nil {
		refChange = getRefChanges(s.Refs, y.Refs)
	}

	// Check the annotated tags for changes
	var annTagChange *ChangeResult
	if s.Tags != nil {
		annTagChange = getAnnTagChanges(s.Tags, y.Tags)
	}

	return &Changes{
		RefChange:    refChange,
		AnnTagChange: annTagChange,
	}
}
