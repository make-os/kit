package plumbing

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/types"
	types2 "gitlab.com/makeos/mosdef/types"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// PackCommit is like PackCommitObject but accepts a commit hash
func PackCommit(repo types.LocalRepo, commitHash plumbing.Hash) (io.Reader, error) {

	// Get the commit object
	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return nil, err
	}

	return PackCommitObject(repo, commit)
}

// CommitPacker describes a function for packing a commit, its tree and blobs into a packfile
type CommitPacker func(repo types.LocalRepo, commit *object.Commit) (io.Reader, error)

// PackCommitObject creates a packfile from the given commit object.
// Caller must ensure commit exist in the repo's object database.
func PackCommitObject(repo types.LocalRepo, commit *object.Commit) (io.Reader, error) {

	// Get a list of all objects that make up the commit.
	var objList = []plumbing.Hash{commit.Hash, commit.TreeHash}

	// Add tree entries hash
	tree, err := commit.Tree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get commit tree")
	}
	for _, entry := range tree.Entries {
		objList = append(objList, entry.Hash)
	}

	// Create packfile
	var buf = bytes.NewBuffer(nil)
	enc := packfile.NewEncoder(buf, repo.GetStorer(), true)
	_, err = enc.Encode(objList, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encoded objects to pack format")
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// UnpackCallback is a function for reading and unpacking a packfile object within UnPackfile.
// header is the object header and read is a function for reading the corresponding object.
type UnpackCallback func(header *packfile.ObjectHeader, read func() (object.Object, error)) error

// PackfileUnpacker describes a function for unpacking a packfile.
type PackfileUnpacker func(pack io.ReadSeeker, cb UnpackCallback) (err error)

// UnPackfile iterates through object headers in the given packfile, passing the callback
// the header and a function to read the current object.
//
// The callback can return ErrStopIteration to stop the iteration and exit with a nil error.
// If a different error is returned, then iteration ends with that error returned.
//
// The packfile reader is reset, so it can be reused by the caller.
// The caller is responsible for closing the packfile reader.
func UnPackfile(pack io.ReadSeeker, cb UnpackCallback) (err error) {
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
