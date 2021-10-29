package push

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/make-os/kit/params"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/pkg/errors"
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

// ID returns the ID of the packed references
func (p *PackedReferences) ID() string {
	return util.ToHex(crypto.Blake2b256(util.ToBytes(p)), true)
}

type PackObject struct {
	Type plumbing.ObjectType
	Hash plumbing.Hash
}

func (o *ObjectObserver) OnHeader(uint32) error        { return nil }
func (o *ObjectObserver) OnFooter(plumbing.Hash) error { return nil }

// ObjectObserver implements packfile.Observer. It allows us to read objects
// of a packfile and also set limitation of blob size.
type ObjectObserver struct {
	Objects     []*PackObject
	MaxBlobSize int64
	totalSize   int64
}

// OnInflatedObjectHeader implements packfile.Observer.
// Returns error if a blob object size surpasses maxBlobSize.
func (o *ObjectObserver) OnInflatedObjectHeader(t plumbing.ObjectType, objSize int64, _ int64) error {
	if t == plumbing.BlobObject && objSize > o.MaxBlobSize {
		return fmt.Errorf("size error: a file's size exceeded the network limit")
	}
	o.Objects = append(o.Objects, &PackObject{Type: t})
	o.totalSize = o.totalSize + objSize
	return nil
}

// OnInflatedObjectContent implements packfile.Observer.
func (o *ObjectObserver) OnInflatedObjectContent(h plumbing.Hash, _ int64, _ uint32, _ []byte) error {
	o.Objects[len(o.Objects)-1].Hash = h
	return nil
}

// PushedObjects is a collection of PackObject
type PushedObjects []*PackObject

// Hashes returns the string equivalent of the object hashes
func (po *PushedObjects) Hashes() (objs []string) {
	for _, o := range *po {
		objs = append(objs, o.Hash.String())
	}
	return
}

// Reader inspects push data from git client, extracting data such as the
// pushed references, objects and object to reference mapping. It also pipes the
// pushed stream to a destination (git-receive-pack) when finished.
type Reader struct {
	dst          io.WriteCloser
	packFile     *os.File
	References   PackedReferences
	Objects      PushedObjects
	repo         plumbing2.LocalRepo
	request      *packp.ReferenceUpdateRequest
	updateReqCB  func(ur *packp.ReferenceUpdateRequest) error
	totalObjSize int64
}

// NewPushReader creates an instance of PushReader, and after inspection, the
// written content will be copied to dst.
func NewPushReader(dst io.WriteCloser, repo plumbing2.LocalRepo) (*Reader, error) {
	packFile, err := ioutil.TempFile(os.TempDir(), "pack")
	if err != nil {
		return nil, err
	}

	return &Reader{
		dst:        dst,
		packFile:   packFile,
		repo:       repo,
		Objects:    []*PackObject{},
		References: make(map[string]*PackedReferenceObject),
	}, nil
}

// Write implements the io.Writer interface.
func (r *Reader) Write(p []byte) (int, error) {
	return r.packFile.Write(p)
}

// UseReferenceUpdateRequestRead sets a callback that is called after the
// push requested has been decoded but yet to be written to git.
// If the callback returns an error, the push request is aborted.
func (r *Reader) UseReferenceUpdateRequestRead(cb func(ur *packp.ReferenceUpdateRequest) error) {
	r.updateReqCB = cb
}

// SetUpdateRequest sets the reference update request
func (r *Reader) SetUpdateRequest(request *packp.ReferenceUpdateRequest) {
	r.request = request
}

// GetUpdateRequest returns the reference update request object
func (r *Reader) GetUpdateRequest() *packp.ReferenceUpdateRequest {
	return r.request
}

// Read reads the packfile, extracting object and reference information
// and finally writes the read data to a provided destination
func (r *Reader) Read() error {

	var err error

	// Seek to the beginning of the packfile
	_, _ = r.packFile.Seek(0, 0)

	// Decode the packfile into a ReferenceUpdateRequest
	r.request = packp.NewReferenceUpdateRequest()
	if err = r.request.Decode(r.packFile); err != nil {
		return errors.Wrap(err, "failed to decode request pack")
	}

	// Extract references from the packfile
	r.References = r.getReferences(r.request)

	// Call updateReqCB callback method
	if r.updateReqCB != nil {
		if err = r.updateReqCB(r.request); err != nil {
			return err
		}
	}

	var scn *packfile.Scanner

	// Confirm if the next 4 bytes say 'PACK', otherwise, the packfile is invalid
	packSig := make([]byte, 4)
	_, _ = r.packFile.Read(packSig)
	if string(packSig) != "PACK" {
		goto writeInput
	}
	_, _ = r.packFile.Seek(-4, 1)

	// Read the packfile
	scn = packfile.NewScanner(r.packFile)
	defer scn.Close()
	r.Objects, err = r.getObjects(scn)
	if err != nil {
		return err
	}

	// Copy to git input stream
writeInput:
	defer r.packFile.Close()
	defer r.dst.Close()
	defer os.Remove(r.packFile.Name())

	_, _ = r.packFile.Seek(0, 0)
	if _, err = io.Copy(r.dst, r.packFile); err != nil {
		return err
	}

	return nil
}

// GetSizeObjects returns the size of pushed objects
func (r *Reader) GetSizeObjects() int64 {
	return r.totalObjSize
}

// getObjects returns a list of objects in the packfile.
// Will return error if an object's size exceeds the allowed max. file size in a push operation.
func (r *Reader) getObjects(scanner *packfile.Scanner) (objs []*PackObject, err error) {
	objObserver := &ObjectObserver{MaxBlobSize: int64(params.MaxPushFileSize)}
	packfileParser, err := packfile.NewParserWithStorage(scanner, r.repo.GetStorer(), objObserver)
	if err != nil {
		return nil, err
	}
	if _, err := packfileParser.Parse(); err != nil {
		return nil, err
	}
	r.totalObjSize = objObserver.totalSize
	return objObserver.Objects, nil
}

// getReferences returns the references found in the pack buffer
func (r *Reader) getReferences(ur *packp.ReferenceUpdateRequest) (references map[string]*PackedReferenceObject) {
	references = make(map[string]*PackedReferenceObject)
	for _, cmd := range ur.Commands {
		references[cmd.Name.String()] = &PackedReferenceObject{
			OldHash: cmd.Old.String(),
			NewHash: cmd.New.String(),
		}
	}
	return
}
