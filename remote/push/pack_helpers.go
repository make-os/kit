package push

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	pushtypes "github.com/themakeos/lobe/remote/push/types"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
)

// ReferenceUpdateRequestPackMaker describes a function for create git
// packfile from a push note and a target repository
type ReferenceUpdateRequestPackMaker func(tx pushtypes.PushNote) (io.ReadSeeker, error)

// MakeReferenceUpdateRequestPack creates a reference update request
// to send to git-receive-pack program.
func MakeReferenceUpdateRequestPack(note pushtypes.PushNote) (io.ReadSeeker, error) {

	var targetRepo = note.GetTargetRepo()
	var hashes []plumbing.Hash

	// Gather objects introduced by the pushed references.
	// We need the list of objects to create a packfile later.
	for _, ref := range note.GetPushedReferences() {
		newHash := ref.NewHash

		// Skip delete request
		if plumbing2.IsZeroHash(newHash) {
			continue
		}

		hashes = append(hashes, plumbing.NewHash(ref.NewHash))
	}

	// Use the hashes to create a packfile
	var pack = bytes.NewBuffer(nil)
	enc := packfile.NewEncoder(pack, targetRepo.GetStorer(), true)
	_, err := enc.Encode(hashes, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack pushed references new hash")
	}

	// Create the request capabilities
	caps := capability.NewList()
	caps.Add(capability.Agent, capability.DefaultAgent)
	caps.Add(capability.ReportStatus)
	caps.Add(capability.OFSDelta)
	caps.Add(capability.DeleteRefs)
	caps.Add(capability.Atomic)

	// Create reference update request using the capabilities and packfile
	ru := packp.NewReferenceUpdateRequestFromCapabilities(caps)
	ru.Packfile = ioutil.NopCloser(pack)
	for _, ref := range note.GetPushedReferences() {
		ru.Commands = append(ru.Commands, &packp.Command{
			Name: plumbing.ReferenceName(ref.Name),
			Old:  plumbing.NewHash(ref.OldHash),
			New:  plumbing.NewHash(ref.NewHash),
		})
	}

	var buf = bytes.NewBuffer(nil)
	if err := ru.Encode(buf); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// GetSizeOfObjects returns the size of objects required to fulfil the push note.
func GetSizeOfObjects(note pushtypes.PushNote) (uint64, error) {
	repo := note.GetTargetRepo()
	if repo == nil {
		return 0, fmt.Errorf("repo is required")
	}

	var total uint64
	for _, ref := range note.GetPushedReferences() {
		err := plumbing2.WalkBack(repo, ref.NewHash, ref.OldHash, func(hash string) error {
			size, err := repo.GetObjectSize(hash)
			if err != nil {
				return err
			}
			total += uint64(size)
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}
