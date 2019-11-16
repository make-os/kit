package repo

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type kvOption struct {
	Key   string
	Value interface{}
}

func getKVOpt(key string, options []kvOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

func prefixOpt(val string) kvOption {
	return kvOption{Key: "prefix", Value: val}
}

func changesOpt(ch *Changes) kvOption {
	return kvOption{Key: "changes", Value: ch}
}

// Repo represents a git repository
type Repo struct {
	*git.Repository
	*GitOps
	Path string
}

// ObjectsID stores the id of all objects in a repo
type ObjectsID map[string]struct{}

// GetSize returns the total size of the objects in the repo.
// Returns error if an object is not found in the repo
func (o *ObjectsID) GetSize(repo *Repo) (int64, error) {
	var size int64
	for hash := range *o {
		obj, err := repo.Object(plumbing.AnyObject, plumbing.NewHash(hash))
		if err != nil {
			return 0, err
		}
		encoded := &plumbing.MemoryObject{}
		if err := obj.Encode(encoded); err != nil {
			return 0, err
		}
		size += encoded.Size()
	}
	return size, nil
}

// getObjectsID returns a map of all object ids
func getObjectsID(repo *Repo) ObjectsID {
	var ids = make(map[string]struct{})
	oi, _ := repo.Objects()
	oi.ForEach(func(o object.Object) error {
		ids[o.ID().String()] = struct{}{}
		return nil
	})
	return ids
}

// getSizeOfChanges calculates the size of the new objects
func getSizeOfChanges(oldState *State, changes *Changes) {

}
