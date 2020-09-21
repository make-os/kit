package plumbing

import (
	"bytes"
	"fmt"
	"io"

	"github.com/make-os/lobe/remote/types"
	types2 "github.com/make-os/lobe/types"
	io2 "github.com/make-os/lobe/util/io"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	ErrFailedToGetTagPointedObject = fmt.Errorf("failed to get pointed object of tag")
)

// GetPackableObjects gets objects related to the given object for creating object-specific packfiles.
// Commit -> commit object, tree object, tree entries object.
// Blob   -> blob object.
// Tree   -> tree object, tree entries object.
// Tag    -> tag object, pointed object [, objects of pointed object].
func GetPackableObjects(
	repo types.LocalRepo,
	obj object.Object,
	objFilter ...func(hash plumbing.Hash) bool) (objs []plumbing.Hash, err error) {

	// Set default object filter function if not defined by caller
	objFilterFunc := func(plumbing.Hash) bool { return true }
	if len(objFilter) > 0 {
		objFilterFunc = objFilter[0]
	}

	// Define a function for collecting only objects that the object filter returns true for.
	appendHash := func(hashes ...plumbing.Hash) {
		for _, h := range hashes {
			if objFilterFunc(h) {
				objs = append(objs, h)
			}
		}
	}

	switch obj.Type() {
	case plumbing.CommitObject:
		// For commit object: Add the commit hash, the tree hash and tree entries
		commit, _ := obj.(*object.Commit)
		objs = append(objs, commit.Hash)
		tree, err := commit.Tree()
		if err != nil {
			return nil, err
		}

		res, err := GetPackableObjects(repo, tree)
		if err != nil {
			return nil, err
		}
		appendHash(res...)

	case plumbing.TreeObject:
		// For tree object: Add the tree hash and its entries hash
		tree, _ := obj.(*object.Tree)
		appendHash(tree.Hash)

		for _, entry := range tree.Entries {

			if entry.Mode.IsFile() {
				appendHash(entry.Hash)
				continue
			}

			// If entry is a tree, traverse it too
			if entry.Mode == filemode.Dir {
				tree, err := repo.GetObject(entry.Hash.String())
				if err != nil {
					return nil, err
				}
				res, err := GetPackableObjects(repo, tree)
				if err != nil {
					return nil, err
				}
				appendHash(res...)
			}
		}

	case plumbing.BlobObject:
		// For blob object: Add the blob hash
		blob, _ := obj.(*object.Blob)
		appendHash(blob.Hash)

	case plumbing.TagObject:
		// For tag object: Add the tag hash and process the pointed
		// object according to type
		tag, _ := obj.(*object.Tag)
		appendHash(tag.Hash)

		switch tag.TargetType {
		case plumbing.CommitObject:
			commit, err := tag.Commit()
			if err != nil {
				return nil, ErrFailedToGetTagPointedObject
			}
			res, err := GetPackableObjects(repo, commit)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get objects of pointed commit")
			}
			appendHash(res...)

		case plumbing.TagObject:
			nestedTag, err := tag.Object()
			if err != nil {
				return nil, ErrFailedToGetTagPointedObject
			}
			res, err := GetPackableObjects(repo, nestedTag)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get objects of pointed tag")
			}
			appendHash(res...)

		case plumbing.BlobObject:
			blob, err := tag.Blob()
			if err != nil {
				return nil, ErrFailedToGetTagPointedObject
			}
			res, err := GetPackableObjects(repo, blob)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get objects of pointed blob")
			}
			appendHash(res...)
		case plumbing.TreeObject:
			tree, err := tag.Tree()
			if err != nil {
				return nil, ErrFailedToGetTagPointedObject
			}
			res, err := GetPackableObjects(repo, tree)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get objects of pointed tree")
			}
			appendHash(res...)
		}
	default:
		return nil, fmt.Errorf("unsupported object type")
	}

	return objs, nil
}

// PackObjectArgs contains arguments for PackObject.
type PackObjectArgs struct {
	// Obj is the target object to pack
	Obj object.Object

	// Filter selects objects that should be packed by returning true.
	Filter func(hash plumbing.Hash) bool
}

// CommitPacker describes a function for packing an object into a packfile.
type CommitPacker func(repo types.LocalRepo, args *PackObjectArgs) (io.Reader, []plumbing.Hash, error)

// PackObject creates a packfile from the given object.
// A commit is packed along with its tree and blobs.
// An annotated tag is packed along with its referenced commit object.
// Caller must ensure commit exist in the repo's object database.
func PackObject(repo types.LocalRepo, args *PackObjectArgs) (pack io.Reader, objs []plumbing.Hash, err error) {

	// Set default object filter if caller did not set it
	if args.Filter == nil {
		args.Filter = func(plumbing.Hash) bool { return true }
	}

	// now := time.Now()
	objs, err = GetPackableObjects(repo, args.Obj, args.Filter)
	if err != nil {
		return nil, nil, err
	}

	var buf = bytes.NewBuffer(nil)
	enc := packfile.NewEncoder(buf, repo.GetStorer(), true)
	_, err = enc.Encode(objs, 0)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to encoded objects to pack format")
	}

	return bytes.NewReader(buf.Bytes()), objs, nil
}

// UnpackCallback is a function for reading and unpacking a packfile object within UnpackPackfile.
// header is the object header and read is a function for reading the corresponding object.
type UnpackCallback func(header *packfile.ObjectHeader, read func() (object.Object, error)) error

// PackfileUnpacker describes a function for unpacking a packfile.
type PackfileUnpacker func(pack io.ReadSeeker, cb UnpackCallback) (err error)

// UnpackPackfile iterates through object headers in the given packfile, passing the callback
// the header and a function to read the current object.
//
// The callback can return ErrExit to stop the iteration and exit with a nil error.
// If a different error is returned, then iteration ends with that error returned.
//
// The packfile reader is reset, so it can be reused by the caller.
// The caller is responsible for closing the packfile reader.
func UnpackPackfile(pack io.ReadSeeker, cb UnpackCallback) (err error) {

	if pack == nil {
		return fmt.Errorf("pack is nil")
	}
	defer pack.Seek(0, 0)

	// Scan the packfile
	scn := packfile.NewScanner(pack)
	_, numObjs, err := scn.Header()
	if err != nil {
		return errors.Wrap(err, "bad packfile")
	}

	// For each objects, decode to object.Object
	for i := uint32(0); i < numObjs; i++ {
		h, err := scn.NextObjectHeader()
		if err != nil {
			return errors.Wrap(err, "bad object header")
		}

		// objReader reads the next object from the scanner
		var objReader = func() (object.Object, error) {
			var memObj plumbing.MemoryObject
			_, _, err = scn.NextObject(&memObj)
			if err != nil {
				return nil, errors.Wrap(err, "failed to write object")
			}
			memObj.SetType(h.Type)
			memObj.SetSize(h.Length)

			obj, err := decodeObject(&memObj)
			if err != nil {
				return nil, errors.Wrap(err, "failed to decode to object")
			}
			return obj, nil
		}

		if cb == nil {
			return nil
		}
		if err2 := cb(h, objReader); err2 != nil {
			if err2 == types2.ErrExit {
				return nil
			}
			return err2
		}
	}

	return
}

// decodeObject takes a raw object stream and returns a git object
func decodeObject(o plumbing.EncodedObject) (object.Object, error) {
	switch o.Type() {
	case plumbing.CommitObject:
		var obj object.Commit
		if err := obj.Decode(o); err != nil {
			return nil, err
		}
		return &obj, nil
	case plumbing.TreeObject:
		var obj object.Tree
		if err := obj.Decode(o); err != nil {
			return nil, err
		}
		return &obj, nil
	case plumbing.BlobObject:
		var obj object.Blob
		if err := obj.Decode(o); err != nil {
			return nil, err
		}
		return &obj, nil
	case plumbing.TagObject:
		var obj object.Tag
		if err := obj.Decode(o); err != nil {
			return nil, err
		}
		return &obj, nil
	default:
		return nil, plumbing.ErrInvalidType
	}
}

// PackToRepoUnpacker describes a function for writing a packfile into a repository
type PackToRepoUnpacker func(repo types.LocalRepo, pack io2.ReadSeekerCloser) error

// UnpackPackfileToRepo unpacks the packfile into the given repository.
func UnpackPackfileToRepo(repo types.LocalRepo, pack io2.ReadSeekerCloser) error {
	return UnpackPackfile(pack, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
		obj, err := read()
		if err != nil {
			return err
		}

		var memObj plumbing.MemoryObject
		err = obj.Encode(&memObj)
		if err != nil {
			return err
		}

		_, err = repo.GetStorer().SetEncodedObject(&memObj)
		if err != nil {
			return errors.Wrap(err, "failed to write object to repo object database")
		}

		return nil
	})
}

// PackObjectFinder describes a function for finding a given object in a packfile
type PackObjectFinder func(pack io.ReadSeeker, hash string) (res object.Object, err error)

// GetObjectFromPack finds and returns an object from a given pack.
func GetObjectFromPack(pack io.ReadSeeker, hash string) (res object.Object, err error) {
	err = UnpackPackfile(pack, func(header *packfile.ObjectHeader, read func() (object.Object, error)) error {
		obj, err := read()
		if err != nil {
			return err
		}
		if obj.ID().String() == hash {
			res = obj
			return types2.ErrExit

		}
		return nil
	})
	return
}
