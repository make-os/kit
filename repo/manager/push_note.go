package manager

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	plumbing2 "gitlab.com/makeos/mosdef/repo/plumbing"
	repo3 "gitlab.com/makeos/mosdef/repo/repo"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
)

// makePackfileFromPushNote creates a packfile from a PushNote
func makePackfileFromPushNote(repo core.BareRepo, tx *core.PushNote) (io.ReadSeeker, error) {

	var buf = bytes.NewBuffer(nil)
	enc := packfile.NewEncoder(buf, repo.GetHost(), true)

	var hashes []plumbing.Hash
	for _, ref := range tx.References {
		for _, h := range ref.Objects {
			hashes = append(hashes, plumbing.NewHash(h))
		}
	}

	_, err := enc.Encode(hashes, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encoded push note to pack format")
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// ReferenceUpdateRequestMaker describes a function for create git packfile from a push note and a target repository
type ReferenceUpdateRequestMaker func(repo core.BareRepo, tx *core.PushNote) (io.ReadSeeker, error)

// MakeReferenceUpdateRequest creates a git reference update request from a push
// transaction. This is what git push sends to the git-receive-pack.
func MakeReferenceUpdateRequest(repo core.BareRepo, tx *core.PushNote) (io.ReadSeeker, error) {

	// Generate a packFile
	packFile, err := makePackfileFromPushNote(repo, tx)
	if err != nil {
		return nil, err
	}

	caps := capability.NewList()
	caps.Add(capability.Agent, "git/2.x")
	caps.Add(capability.ReportStatus)
	caps.Add(capability.OFSDelta)
	caps.Add(capability.DeleteRefs)

	ru := packp.NewReferenceUpdateRequestFromCapabilities(caps)
	ru.Packfile = ioutil.NopCloser(packFile)
	for _, ref := range tx.References {
		ru.Commands = append(ru.Commands, &packp.Command{
			Name: plumbing.ReferenceName(ref.Name),
			Old:  plumbing.NewHash(ref.OldHash),
			New:  plumbing.NewHash(ref.NewHash),
		})
	}

	var buf = bytes.NewBuffer(nil)
	if err = ru.Encode(buf); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// makePushNoteFromStateChange creates a PushNote object from changes between two
// states. Only the reference information is set in the PushNote object returned.
func makePushNoteFromStateChange(
	repo core.BareRepo,
	oldState,
	newState core.BareRepoState) (*core.PushNote, error) {

	// Compute the changes between old and new states
	tx := &core.PushNote{References: []*core.PushedReference{}}
	changes := oldState.GetChanges(newState)

	// For each changed references, generate a PushedReference object
	for _, change := range changes.References.Changes {

		newHash := change.Item.GetData()
		var commit *object.Commit
		var err error
		var objHashes []string

		// Get the hash of the old version of the changed reference
		var changedRefOld = oldState.GetReferences().Get(change.Item.GetName())
		var changedRefOldVerHash string
		if changedRefOld != nil {
			changedRefOldVerHash = changedRefOld.GetData()
		}

		// Get the commit object, if changed reference is a branch
		if plumbing2.IsBranch(change.Item.GetName()) {
			commit, err = repo.CommitObject(plumbing.NewHash(newHash))
			if err != nil {
				return nil, err
			}
		}

		// Get the commit referenced by the tag
		if plumbing2.IsTag(change.Item.GetName()) {
			nameParts := strings.Split(change.Item.GetName(), "/")

			// Get the tag from the repository.
			// If we can't find it and the change type is a 'remove', skip to
			// the reference addition section
			tag, err := repo.Tag(nameParts[len(nameParts)-1])
			if err != nil {
				if err == git.ErrTagNotFound && change.Action == core.ChangeTypeRemove {
					goto addRef
				}
				return nil, err
			}

			// Handle annotated object
			to, err := repo.TagObject(tag.Hash())
			if err != nil && err != plumbing.ErrObjectNotFound {
				return nil, err
			} else if to != nil {
				commit, err = to.Commit()
				if err != nil {
					return nil, err
				}

				// Register the tag object as part of the objects updates
				objHashes = append(objHashes, to.Hash.String())

				// If the changed reference has an old version, we also need to
				// get the commit pointed to by the old version and set it as
				// the value of changedRefOldVerHash
				if changedRefOldVerHash != "" {
					oldTag, err := repo.TagObject(plumbing.NewHash(changedRefOldVerHash))
					if err != nil {
						return nil, err
					}
					oldTagCommit, err := oldTag.Commit()
					if err != nil {
						return nil, err
					}
					changedRefOldVerHash = oldTagCommit.Hash.String()
				}

			} else {
				// Handle lightweight tag
				commit, err = repo.CommitObject(tag.Hash())
				if err != nil {
					return nil, err
				}
			}
		}

		// Get the commit referenced by the note
		if plumbing2.IsNote(change.Item.GetName()) {
			commit, err = repo.CommitObject(plumbing.NewHash(newHash))
			if err != nil {
				return nil, err
			}
		}

	addRef:
		// Generate the pushed reference object depending on the type of change
		// that happened to the reference.
		switch change.Action {
		case core.ChangeTypeNew:
			histHashes, err := repo3.GetCommitHistory(repo, commit, "")
			if err != nil {
				return nil, err
			}
			tx.References = append(tx.References, &core.PushedReference{
				Name:    change.Item.GetName(),
				NewHash: newHash,
				OldHash: plumbing.ZeroHash.String(),
				Objects: append(objHashes, histHashes...),
			})

		case core.ChangeTypeUpdate:
			histHashes, err := repo3.GetCommitHistory(repo, commit, changedRefOldVerHash)
			if err != nil {
				return nil, err
			}
			tx.References = append(tx.References, &core.PushedReference{
				Name:    change.Item.GetName(),
				Objects: append(objHashes, histHashes...),
				NewHash: newHash,
				OldHash: oldState.GetReferences().Get(change.Item.GetName()).GetData(),
			})

		case core.ChangeTypeRemove:
			tx.References = append(tx.References, &core.PushedReference{
				Name:    change.Item.GetName(),
				NewHash: plumbing.ZeroHash.String(),
				OldHash: changedRefOldVerHash,
			})
		}
	}

	return tx, nil
}

// MakePackfile creates a git reference update request packfile from state
// changes between old and new repository state.
func MakePackfile(
	repo core.BareRepo,
	oldState,
	newState core.BareRepoState) (io.ReadSeeker, error) {

	pushNote, err := makePushNoteFromStateChange(repo, oldState, newState)
	if err != nil {
		return nil, err
	}

	return MakeReferenceUpdateRequest(repo, pushNote)
}
