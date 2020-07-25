package push

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"gitlab.com/makeos/lobe/remote/types"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
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

type PushedObjects []*PackObject

// Hashes returns the string equivalent of the object hashes
func (po *PushedObjects) Hashes() (objs []string) {
	for _, o := range *po {
		objs = append(objs, o.Hash.String())
	}
	return
}

// PushReader inspects push data from git client, extracting data such as the
// pushed references, objects and object to reference mapping. It also pipes the
// pushed stream to a destination (git-receive-pack) when finished.
type PushReader struct {
	dst         io.WriteCloser
	packFile    *os.File
	buf         []byte
	References  PackedReferences
	Objects     PushedObjects
	repo        types.LocalRepo
	request     *packp.ReferenceUpdateRequest
	updateReqCB func(ur *packp.ReferenceUpdateRequest) error
}

// NewPushReader creates an instance of PushReader, and after inspection, the
// written content will be copied to dst.
func NewPushReader(dst io.WriteCloser, repo types.LocalRepo) (*PushReader, error) {
	packFile, err := ioutil.TempFile(os.TempDir(), "pack")
	if err != nil {
		return nil, err
	}

	return &PushReader{
		dst:        dst,
		packFile:   packFile,
		repo:       repo,
		Objects:    []*PackObject{},
		References: make(map[string]*PackedReferenceObject),
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

// SetUpdateRequest sets the reference update request
func (r *PushReader) SetUpdateRequest(request *packp.ReferenceUpdateRequest) {
	r.request = request
}

// GetUpdateRequest returns the reference update request object
func (r *PushReader) GetUpdateRequest() *packp.ReferenceUpdateRequest {
	return r.request
}

// Read reads the packfile, extracting object and reference information
// and finally writes the read data to a provided destination
func (r *PushReader) Read(gitCmd *exec.Cmd) error {

	var err error

	// Seek to the beginning of the packfile
	r.packFile.Seek(0, 0)

	// Decode the packfile into a ReferenceUpdateRequest
	r.request = packp.NewReferenceUpdateRequest()
	if err = r.request.Decode(r.packFile); err != nil {
		return errors.Wrap(err, "failed to decode request pack")
	}

	// Extract references from the packfile
	r.References = r.getReferences(r.request)

	// Call OnReferenceUpdateRequestRead callback method
	if r.updateReqCB != nil {
		if err = r.updateReqCB(r.request); err != nil {
			return err
		}
	}

	var scn *packfile.Scanner

	// Confirm if the next 4 bytes say 'PACK', otherwise, the packfile is invalid
	packSig := make([]byte, 4)
	r.packFile.Read(packSig)
	if string(packSig) != "PACK" {
		goto write_input
	}
	r.packFile.Seek(-4, 1)

	// Read the packfile
	scn = packfile.NewScanner(r.packFile)
	defer scn.Close()
	r.Objects, err = r.getObjects(scn)
	if err != nil {
		return errors.Wrap(err, "failed to get objects")
	}

	// Copy to git input stream
write_input:
	defer r.packFile.Close()
	defer r.dst.Close()
	defer os.Remove(r.packFile.Name())

	r.packFile.Seek(0, 0)
	if _, err = io.Copy(r.dst, r.packFile); err != nil {
		return err
	}

	// Wait for the git process to finish only if the git command is set
	if gitCmd != nil {
		gitCmd.Process.Wait()
	}

	return nil
}

// getObjects returns a list of objects in the packfile
func (r *PushReader) getObjects(scanner *packfile.Scanner) (objs []*PackObject, err error) {
	objObserver := &ObjectObserver{}
	packfileParser, err := packfile.NewParserWithStorage(scanner, r.repo.GetStorer(), objObserver)
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
