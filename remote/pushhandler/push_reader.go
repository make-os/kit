package pushhandler

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	types2 "gitlab.com/makeos/mosdef/remote/pushpool/types"
	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
)

// PackedReferenceObject represent references added to a pack file
type PackedReferenceObject struct {
	OldHash string
	NewHash string
}

// PackedReferences represents a collection of packed references
type PackedReferences map[string]*PackedReferenceObject

// Names return the Names of the references
func (p *PackedReferences) Names() (refs []string) {
	for name := range *p {
		refs = append(refs, name)
	}
	return
}

type PackObject struct {
	Type plumbing.ObjectType
	Hash plumbing.Hash
}

// objObserver implements packfile.Observer
type ObjectObserver struct {
	objects []*PackObject
}

func (o *ObjectObserver) OnInflatedObjectHeader(t plumbing.ObjectType, objSize int64,
	pos int64) error {
	o.objects = append(o.objects, &PackObject{Type: t})
	return nil
}

func (o *ObjectObserver) OnInflatedObjectContent(h plumbing.Hash, pos int64,
	crc uint32, content []byte) error {
	o.objects[len(o.objects)-1].Hash = h
	return nil
}

func (o *ObjectObserver) OnHeader(count uint32) error    { return nil }
func (o *ObjectObserver) OnFooter(h plumbing.Hash) error { return nil }

// PushReader inspects push data from git client, extracting data such as the
// pushed references, objects and object to reference mapping. It also pipes the
// pushed stream to a destination (git-receive-pack) when finished.
type PushReader struct {
	dst           io.WriteCloser
	packFile      *os.File
	buf           []byte
	References    PackedReferences
	Objects       []*PackObject
	ObjectsRefs   ObjRefMap
	repo          types2.LocalRepo
	refsUpdateReq *packp.ReferenceUpdateRequest
	updateReqCB   func(ur *packp.ReferenceUpdateRequest) error
}

// NewPushReader creates an instance of PushReader, and after inspection, the
// written content will be copied to dst.
func NewPushReader(dst io.WriteCloser, repo types2.LocalRepo) (*PushReader, error) {
	packFile, err := ioutil.TempFile(os.TempDir(), "pack")
	if err != nil {
		return nil, err
	}

	return &PushReader{
		dst:         dst,
		packFile:    packFile,
		repo:        repo,
		ObjectsRefs: make(map[string][]string),
		Objects:     []*PackObject{},
		References:  make(map[string]*PackedReferenceObject),
	}, nil
}

// Write implements the io.Writer interface.
func (r *PushReader) Write(p []byte) (int, error) {
	return r.packFile.Write(p)
}

// OnReferenceUpdateRequestRead sets a callback that is called after the
// push requested has been decoded but yet to be written to git.
// If the callback returns an error, the push request is aborted.
func (r *PushReader) OnReferenceUpdateRequestRead(cb func(ur *packp.ReferenceUpdateRequest) error) {
	r.updateReqCB = cb
}

// GetUpdateRequest returns the reference update request object
func (r *PushReader) GetUpdateRequest() *packp.ReferenceUpdateRequest {
	return r.refsUpdateReq
}

// Read reads the packfile, extracting object and reference information
// and finally writes the read data to a provided destination
func (r *PushReader) Read() error {

	var err error

	// Seek to the beginning of the packfile
	r.packFile.Seek(0, 0)

	// Decode the packfile into a ReferenceUpdateRequest
	r.refsUpdateReq = packp.NewReferenceUpdateRequest()
	if err = r.refsUpdateReq.Decode(r.packFile); err != nil {
		return errors.Wrap(err, "failed to decode request pack")
	}

	// Extract references from the packfile
	r.References = r.getReferences(r.refsUpdateReq)

	// Call OnReferenceUpdateRequestRead callback method
	if r.updateReqCB != nil {
		if err = r.updateReqCB(r.refsUpdateReq); err != nil {
			return err
		}
	}

	// Scan the packfile and extract objects hashes.
	// Confirm if the next 4 bytes are indeed 'PACK', otherwise, the packfile is invalid
	packSig := make([]byte, 4)
	r.packFile.Read(packSig)
	if string(packSig) != "PACK" {
		return r.done()
	}
	r.packFile.Seek(-4, 1)

	// Read the packfile
	scn := packfile.NewScanner(r.packFile)
	defer scn.Close()
	r.Objects, err = r.getObjects(scn)
	if err != nil {
		return errors.Wrap(err, "failed to get objects")
	}

	return r.done()
}

// getObjects returns a list of objects in the packfile
func (r *PushReader) getObjects(scanner *packfile.Scanner) (objs []*PackObject, err error) {
	objObserver := &ObjectObserver{}
	packfileParser, err := packfile.NewParserWithStorage(scanner, r.repo.GetHost(), objObserver)
	if err != nil {
		return nil, err
	}
	if _, err := packfileParser.Parse(); err != nil {
		return nil, err
	}
	return objObserver.objects, nil
}

// getReferences returns the references found in the pack buffer
func (r *PushReader) getReferences(ur *packp.ReferenceUpdateRequest) (references map[string]*PackedReferenceObject) {
	references = make(map[string]*PackedReferenceObject)
	for _, cmd := range ur.Commands {
		references[cmd.Name.String()] = &PackedReferenceObject{
			OldHash: cmd.Old.String(),
			NewHash: cmd.New.String(),
		}
	}
	return
}

// done copies the written content from the inspector to dst and closes the
// destination and source readers and creates a mapping of objects to references.
func (r *PushReader) done() (err error) {

	r.packFile.Seek(0, 0)
	if _, err = io.Copy(r.dst, r.packFile); err != nil {
		return
	}

	if err = r.packFile.Close(); err != nil {
		return
	}

	if err = r.dst.Close(); err != nil {
		return
	}

	// Give git some time to process the input
	time.Sleep(100 * time.Millisecond)

	r.ObjectsRefs, err = r.mapObjectsToRef()
	if err != nil {
		return errors.Wrap(err, "failed to map objects to references")
	}

	os.Remove(r.packFile.Name())

	return
}

// ObjRefMap maps objects to the references they belong to.
type ObjRefMap map[string][]string

// RemoveRef removes a reference from the list of references an object belongs to
func (m *ObjRefMap) RemoveRef(objHash, ref string) error {
	refs, ok := (*m)[objHash]
	if !ok {
		return fmt.Errorf("object not found")
	}
	var newRefs []string
	for _, r := range refs {
		if r != ref {
			newRefs = append(newRefs, r)
		}
	}
	(*m)[objHash] = newRefs
	return nil
}

// getObjects returns a list of objects that map to the given ref
func (m *ObjRefMap) GetObjectsOf(ref string) (objs []string) {
	for obj, refs := range *m {
		if funk.ContainsString(refs, ref) {
			objs = append(objs, obj)
		}
	}
	return
}

// mapObjectsToRef returns a map that pairs pushed objects to one or more
// repository references they belong to.
func (r *PushReader) mapObjectsToRef() (ObjRefMap, error) {
	var mappings = make(map[string][]string)

	if len(r.Objects) == 0 {
		return mappings, nil
	}

	for _, ref := range r.References.Names() {
		var entries []string
		var err error

		refObj, err := r.repo.Reference(plumbing.ReferenceName(ref), true)
		if err != nil {
			return nil, err
		}

		obj, err := r.repo.Object(plumbing.AnyObject, refObj.Hash())
		if err != nil {
			return nil, err
		}

		objType := obj.Type()

		if objType == plumbing.CommitObject {
			entries, err = repo2.GetCommitHistory(r.repo, obj.(*object.Commit), "")
			if err != nil {
				return nil, err
			}
		}

		if objType == plumbing.TagObject {
			commit, err := obj.(*object.Tag).Commit()
			if err != nil {
				return nil, err
			}
			entries, err = repo2.GetCommitHistory(r.repo, commit, "")
			if err != nil {
				return nil, err
			}
			entries = append(entries, obj.(*object.Tag).ID().String())
		}

		for _, obj := range r.Objects {
			if funk.ContainsString(entries, obj.Hash.String()) {
				objRefs, ok := mappings[obj.Hash.String()]
				if !ok {
					objRefs = []string{}
				}
				if !funk.ContainsString(objRefs, ref) {
					objRefs = append(objRefs, ref)
				}
				mappings[obj.Hash.String()] = objRefs

			}
		}
	}

	return mappings, nil
}
