package repo

import (
	"github.com/makeos/mosdef/util"
)

type (
	// ColChangeType describes a change to a collection item
	ColChangeType int
)

const (
	// ColChangeTypeNew represents a new, unique item added to a collection
	ColChangeTypeNew = iota
	// ColChangeTypeRemove represents a removal of a collection item
	ColChangeTypeRemove
	// ColChangeTypeUpdate represents an update to the value of a collection item
	ColChangeTypeUpdate
)

// Item represents a git object or reference
type Item interface {
	GetName() string
	Equal(o interface{}) bool
	GetData() string
	GetType() string
}

// Items represents a collection of git objects or references identified by a name
type Items interface {
	Has(name interface{}) bool
	Get(name interface{}) Item
	Equal(o interface{}) bool
	ForEach(func(i Item) bool)
	Len() int64
	Bytes() []byte
	Hash() util.Hash
}

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
	items map[string]*Obj
}

// NewObjCol creates an ObjCol instance
func NewObjCol(r map[string]*Obj) *ObjCol {
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
	// util.ObjectToBytes apply map key sorting
	var mapI = make(map[string]interface{}, len(oc.items))
	for k, v := range oc.items {
		mapI[k] = v
	}
	return util.ObjectToBytes(mapI)
}

// Hash returns 32-bytes blake2b hash of the collection
func (oc *ObjCol) Hash() util.Hash {
	return util.BytesToHash(util.Blake2b256(oc.Bytes()))
}

// ChangeResult includes information about changes
type ChangeResult struct {
	SizeChange bool
	Changes    []*ItemChange
}

// ItemChange describes a change event
type ItemChange struct {
	Item   Item
	Action ColChangeType
}

func newChange(i Item, action ColChangeType) *ItemChange {
	return &ItemChange{Item: i, Action: action}
}

// getChanges takes one old collection of items and an updated collection of
// items and attempts to determine the changes that must be executed against
// the old collection before it is equal to the updated collection.
func getChanges(old, update Items) *ChangeResult {

	// pp.Println("OLD", old)
	// pp.Println("Update", update)

	var result = new(ChangeResult)
	if update == nil {
		return result
	}

	// Detect size change between the collections.
	// If size is not the same, set SizeChange to true
	if old.Len() != update.Len() {
		result.SizeChange = true
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
			result.Changes = append(result.Changes, newChange(curItem, ColChangeTypeNew))
			return false
		}

		// If the longer pointer is the old collection, and the current item
		// is not in the shorter collection (updated collection), it means the
		// current was removed in the updated collection.
		if !updateIsLonger && curItemInShorter == nil {
			result.Changes = append(result.Changes, newChange(curItem, ColChangeTypeRemove))
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
			result.Changes = append(result.Changes, newChange(updRef, ColChangeTypeUpdate))
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
			result.Changes = append(result.Changes, newChange(curNewRef, ColChangeTypeNew))
			return false
		})
	}

	return result
}

// getRefChanges returns the reference changes from old to upd.
func getRefChanges(old, update *ObjCol) *ChangeResult {
	return getChanges(old, update)
}

// getAnnTagChanges returns the annotated tag changes from old to upd.
func getAnnTagChanges(old, update *ObjCol) *ChangeResult {
	return getChanges(old, update)
}

// Changes describes reference changes that happened to a repository
// from a previous state to its current state.
type Changes struct {
	RefChange    *ChangeResult
	AnnTagChange *ChangeResult
}
