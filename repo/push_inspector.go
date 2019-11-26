package repo

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// packedReferenceObject represent references added to a pack file
type packedReferenceObject struct {
	name      string
	startHash string
	endHash   string
}

// packedReferences represents a collection of packed references
type packedReferences []*packedReferenceObject

// names return the names of the references
func (p *packedReferences) names() (refs []string) {
	for _, p := range *p {
		refs = append(refs, p.name)
	}
	return
}

// PushInspector inspects push data from git client, extracting data such as the
// pushed references, objects and object to reference mapping. It also pipes the
// pushed stream to a destination (git-receive-pack) when finished.
type PushInspector struct {
	dst          io.WriteCloser
	packFile     *os.File
	buf          []byte
	references   packedReferences
	objects      []*packObject
	objectRefMap objRefMap
	repo         *Repo
}

type packObject struct {
	Type plumbing.ObjectType
	Hash plumbing.Hash
}

type objectObserver struct {
	objects []*packObject
}

func (o *objectObserver) OnInflatedObjectHeader(t plumbing.ObjectType, objSize int64,
	pos int64) error {
	o.objects = append(o.objects, &packObject{Type: t})
	return nil
}

func (o *objectObserver) OnInflatedObjectContent(h plumbing.Hash, pos int64,
	crc uint32, content []byte) error {
	o.objects[len(o.objects)-1].Hash = h
	return nil
}

func (o *objectObserver) OnHeader(count uint32) error    { return nil }
func (o *objectObserver) OnFooter(h plumbing.Hash) error { return nil }

// newPushInspector creates an instance of PushInspector, and after inspection, the
// written content will be copied to dst.
func newPushInspector(dst io.WriteCloser, repo *Repo) (*PushInspector, error) {
	packFile, err := ioutil.TempFile(os.TempDir(), "pack")
	if err != nil {
		return nil, err
	}

	return &PushInspector{
		dst:      dst,
		packFile: packFile,
		repo:     repo,
	}, nil
}

// Write implements the io.Writer interface.
func (pi *PushInspector) Write(p []byte) (int, error) {
	return pi.packFile.Write(p)
}

// Inspect reads the packfile, extracting object and reference information
func (pi *PushInspector) Inspect() error {

	pi.packFile.Seek(0, 0)

	// Read the packlines and get specific information from it.
	scn := pktline.NewScanner(pi.packFile)
	pi.references = append(pi.references, pi.getReferences(scn)...)

	// Since the packline scanner stops after unable to parse 'PACK',
	// we have to go back 4 bytes backwards before we start scanning the pack file.
	pi.packFile.Seek(-4, 1)

	// Read the pack file
	var err error
	packfileScn := packfile.NewScanner(pi.packFile)
	defer packfileScn.Close()
	pi.objects, err = pi.getObjects(packfileScn)
	if err != nil {
		return errors.Wrap(err, "failed to get objects")
	}

	return pi.done()
}

// getObjects returns a list of objects in the packfile
func (pi *PushInspector) getObjects(scanner *packfile.Scanner) (objs []*packObject, err error) {
	objObserver := &objectObserver{}
	packfileParser, err := packfile.NewParser(scanner, objObserver)
	if err != nil {
		return nil, err
	}
	if _, err := packfileParser.Parse(); err != nil {
		return nil, err
	}
	return objObserver.objects, nil
}

// getReferences returns the references found in the pack buffer
func (pi *PushInspector) getReferences(scanner *pktline.Scanner) (references []*packedReferenceObject) {
	for {
		if !scanner.Scan() {
			break
		}
		pkLine := strings.Fields(string(scanner.Bytes()))
		if len(pkLine) > 0 {
			refObj := &packedReferenceObject{
				name:      strings.Trim(pkLine[2], "\x00"),
				startHash: pkLine[0],
				endHash:   pkLine[1],
			}
			references = append(references, refObj)
		}
	}
	return
}

// done copies the written content from the inspector to dst and closes the
// destination and source readers and creates a mapping of objects to references.
func (pi *PushInspector) done() (err error) {

	pi.packFile.Seek(0, 0)
	if _, err = io.Copy(pi.dst, pi.packFile); err != nil {
		return
	}

	if err = pi.packFile.Close(); err != nil {
		return
	}

	if err = pi.dst.Close(); err != nil {
		return
	}

	// Give git some time to process the input
	time.Sleep(100 * time.Millisecond)

	pi.objectRefMap, err = pi.mapObjectsToRef()
	if err != nil {
		return errors.Wrap(err, "failed to map objects to references")
	}

	return
}

type objRefMap map[string][]string

func (m *objRefMap) removeRef(objHash, ref string) error {
	refs, ok := (*m)[objHash]
	if !ok {
		return fmt.Errorf("object not found")
	}
	newRefs := []string{}
	for _, r := range refs {
		if r != ref {
			newRefs = append(newRefs, r)
		}
	}
	(*m)[objHash] = newRefs
	return nil
}

// mapObjectsToRef returns a map that pairs pushed objects to one or more
// repository references they belong to.
func (pi *PushInspector) mapObjectsToRef() (objRefMap, error) {
	var mappings = make(map[string][]string)

	if len(pi.objects) == 0 {
		return mappings, nil
	}

	for _, ref := range pi.references.names() {
		var entries []string
		var err error

		refObj, err := pi.repo.Reference(plumbing.ReferenceName(ref), true)
		if err != nil {
			return nil, err
		}

		obj, err := pi.repo.Object(plumbing.AnyObject, refObj.Hash())
		if err != nil {
			return nil, err
		}

		objType := obj.Type()

		if objType == plumbing.CommitObject {
			entries, err = getCommitHistory(pi.repo, obj.(*object.Commit), "")
			if err != nil {
				return nil, err
			}
		}

		if objType == plumbing.TagObject {
			commit, err := obj.(*object.Tag).Commit()
			if err != nil {
				return nil, err
			}
			entries, err = getCommitHistory(pi.repo, commit, "")
			if err != nil {
				return nil, err
			}
			entries = append(entries, obj.(*object.Tag).ID().String())
		}

		for _, obj := range pi.objects {
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
