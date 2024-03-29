package plumbing

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
)

// Obj implements Item. It represents a repository item.
type Obj struct {
	Type string
	Name string
	Data string
}

// GetName returns the name
func (ob *Obj) GetName() string {
	return ob.Name
}

// GetData returns the data
func (ob *Obj) GetData() string {
	return ob.Data
}

// GetType returns the type
func (ob *Obj) GetType() string {
	return ob.Type
}

// Equal checks whether r and o are equal
func (ob *Obj) Equal(o interface{}) bool {
	return ob.Name == o.(*Obj).Name &&
		ob.Data == o.(*Obj).Data &&
		ob.Type == o.(*Obj).Type
}

// ObjCol implements Items. It is a collection of objects.
type ObjCol struct {
	items map[string]Item
}

// NewObjCol creates an ObjCol instance
func NewObjCol(r map[string]Item) *ObjCol {
	return &ObjCol{items: r}
}

// Has returns true if an object by the given name exists
func (oc *ObjCol) Has(name interface{}) bool {
	return oc.items[name.(string)] != nil
}

// Get gets an object by name
func (oc *ObjCol) Get(name interface{}) Item {
	res, ok := oc.items[name.(string)]
	if !ok {
		return nil
	}
	return res
}

// Equal checks whether oc and o are equal
func (oc *ObjCol) Equal(o interface{}) bool {
	if len(oc.items) != len(o.(*ObjCol).items) {
		return false
	}
	for name, ref := range oc.items {
		oRef := o.(*ObjCol)
		if r := oRef.Get(name); r == nil || !r.(*Obj).Equal(ref) {
			return false
		}
	}
	return true
}

// Len returns the size of the collection
func (oc *ObjCol) Len() int64 {
	return int64(len(oc.items))
}

// ForEach iterates through the collection.
// Each item is passed as the only argument to the callback.
// Return true to break immediately
func (oc *ObjCol) ForEach(iteratee func(i Item) bool) {
	for _, v := range oc.items {
		if iteratee(v) {
			break
		}
	}
}

// Bytes serializes the collection
func (oc *ObjCol) Bytes() []byte {
	// Convert items type to map[string]interface{} to enable
	// util.ToBytes apply map key sorting
	var mapI = make(map[string]interface{}, len(oc.items))
	for k, v := range oc.items {
		mapI[k] = v
	}
	return util.ToBytes(mapI)
}

// Hash returns 32-bytes blake2b hash of the collection
func (oc *ObjCol) Hash() util.Bytes32 {
	return util.BytesToBytes32(crypto.Blake2b256(oc.Bytes()))
}

// EmptyChangeResult returns an empty ChangeResult
func EmptyChangeResult() *ChangeResult {
	return &ChangeResult{Changes: []*ItemChange{}}
}

func newChange(i Item, action ColChangeType) *ItemChange {
	return &ItemChange{Item: i, Action: action}
}

// GetChanges takes one old collection of items and an updated collection of
// items and attempts to determine the changes that must be executed against
// the old collection before it is equal to the updated collection.
func GetChanges(old, update Items) *ChangeResult {
	var result = new(ChangeResult)
	if update == nil {
		return EmptyChangeResult()
	}

	// We typically loop through the longest collection
	// to compare with the shorter collection.
	// Here, we determine which is the longer collection.
	longerPtr, shorterPtr := old, update
	if old.Len() < update.Len() {
		longerPtr, shorterPtr = update, old
	}

	// We need to store a flag that tells us if the update
	// collection is the longest
	updateIsLonger := longerPtr.Equal(update)

	// Now, we loop through the longer collection,
	longerPtr.ForEach(func(curItem Item) bool {

		// Get the current item in the shorter collection
		curItemInShorter := shorterPtr.Get(curItem.GetName())

		// If the longer pointer is the updated collection, and the current
		// item is not in the shorter collection, it means the current item is
		// new and unknown to the old collection.
		if updateIsLonger && curItemInShorter == nil {
			result.Changes = append(result.Changes, newChange(curItem, ChangeTypeNew))
			return false
		}

		// If the longer pointer is the old collection, and the current item
		// is not in the shorter collection (updated collection), it means the
		// current was removed in the updated collection.
		if !updateIsLonger && curItemInShorter == nil {
			result.Changes = append(result.Changes, newChange(curItem, ChangeTypeRemove))
			return false
		}

		// At this point, both the old and new collections share the current item.
		// We have to do a deeper equality check to ensure their values are the
		// same; If not, it means the current item shared by the older
		// collection was updated.
		if !curItemInShorter.Equal(curItem) {
			updRef := curItemInShorter
			if updateIsLonger {
				updRef = curItem
			}
			result.Changes = append(result.Changes, newChange(updRef, ChangeTypeUpdate))
		}

		return false
	})

	// When the longer pointer is not the updated collection, add whatever is
	// in the update collection that isn't already in the old collection
	if !updateIsLonger {
		update.ForEach(func(curNewRef Item) bool {
			if old.Has(curNewRef.GetName()) {
				return false
			}
			result.Changes = append(result.Changes, newChange(curNewRef, ChangeTypeNew))
			return false
		})
	}

	return result
}

// getRefChanges returns the reference changes from old to upd.
func getRefChanges(old, update Items) *ChangeResult {
	return GetChanges(old, update)
}

// GetState describes the current state of repository
type State struct {
	util.CodecUtil
	References *ObjCol
}

// GetReferences returns the current repo references
func (s *State) GetReferences() Items {
	return s.References
}

// MakeStateFromItem creates a GetState object from an Item.
// If Item is nil, an empty GetState is returned
func MakeStateFromItem(item Item) *State {
	obj := map[string]Item{}
	if item != nil {
		obj[item.GetName()] = item
	}
	return &State{References: NewObjCol(obj)}
}

// IsEmpty checks whether the state is empty
func (s *State) IsEmpty() bool {
	return s.References.Len() == 0
}

// GetChanges summarizes the changes between GetState s and y.
func (s *State) GetChanges(y RepoRefsState) *Changes {

	var refChange *ChangeResult

	// If y is nil, return an empty change result since
	// there is nothing to compare s with.
	if y == nil {
		return &Changes{References: EmptyChangeResult()}
	}

	// As long as y has a reference collection,
	// we can check for changes
	if s.References != nil {
		refChange = getRefChanges(s.References, y.GetReferences())
	}

	return &Changes{
		References: refChange,
	}
}

// GetRepoState returns the state of the repository
// repo: The target repository
// options: Allows the caller to configure how and what state are gathered
func GetRepoState(repo LocalRepo, options ...KVOption) RepoRefsState {

	refMatch := ""
	if opt := GetKVOpt("match", options); opt != nil {
		refMatch = opt.(string)
	}

	// Get references
	refs := make(map[string]Item)
	if refMatch == "" || strings.HasPrefix(refMatch, "refs") {
		refsI, _ := repo.References()
		refsI.ForEach(func(ref *plumbing.Reference) error {

			// Ignore HEAD reference
			if strings.ToLower(ref.Name().String()) == "head" {
				return nil
			}

			// If a ref match option is set, ignore non-matching reference
			if refMatch != "" && ref.Name().String() != refMatch {
				return nil
			}

			refs[ref.Name().String()] = &Obj{
				Type: "ref",
				Name: ref.Name().String(),
				Data: ref.Hash().String(),
			}

			return nil
		})
	}

	return &State{
		References: NewObjCol(refs),
	}
}
