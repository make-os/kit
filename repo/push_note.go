package repo

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/vmihailenco/msgpack"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
)

// PushNote implements types.PushNote
type PushNote struct {
	targetRepo  types.BareRepo
	RepoName    string                 `json:"repoName" msgpack:"repoName"`       // The name of the repo
	References  types.PushedReferences `json:"references" msgpack:"references"`   // A list of references pushed
	PusherKeyID string                 `json:"pusherKeyId" msgpack:"pusherKeyId"` // The PGP key of the pusher
	Size        uint64                 `json:"size" msgpack:"size"`               // Total size of all objects pushed
	Timestamp   int64                  `json:"timestamp" msgpack:"timestamp"`     // Unix timestamp
	NodeSig     []byte                 `json:"nodeSig" msgpack:"nodeSig"`         // The signature of the node that created the PushNote
	NodePubKey  string                 `json:"nodePubKey" msgpack:"nodePubKey"`   // The public key of the push note signer
}

// GetTargetRepo returns the target repository
func (pt *PushNote) GetTargetRepo() types.BareRepo {
	return pt.targetRepo
}

// GetPusherKeyID returns the pusher gpg key ID
func (pt *PushNote) GetPusherKeyID() string {
	return pt.PusherKeyID
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pt *PushNote) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(pt.RepoName, pt.References, pt.PusherKeyID,
		pt.Size, pt.Timestamp, pt.NodeSig, &pt.NodePubKey)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pt *PushNote) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&pt.RepoName, &pt.References, &pt.PusherKeyID,
		&pt.Size, &pt.Timestamp, &pt.NodeSig, &pt.NodePubKey)
}

// Bytes returns a serialized version of the object
func (pt *PushNote) Bytes() []byte {
	return util.ObjectToBytes(pt)
}

// GetPushedObjects returns all objects from all pushed references
func (pt *PushNote) GetPushedObjects() (objs []string) {
	for _, ref := range pt.GetPushedReferences() {
		objs = append(objs, ref.Objects...)
	}
	return
}

// LenMinusFee returns the length of the serialized tx minus
// the total length of fee fields.
func (pt *PushNote) LenMinusFee() uint64 {
	var feeFieldsLen = 0
	for _, r := range pt.References {
		feeFieldsLen += len(util.ObjectToBytes(r.Fee))
	}

	return pt.Len() - uint64(feeFieldsLen)
}

// GetRepoName returns the name of the repo receiving the push
func (pt *PushNote) GetRepoName() string {
	return pt.RepoName
}

// GetPushedReferences returns the pushed references
func (pt *PushNote) GetPushedReferences() types.PushedReferences {
	return pt.References
}

// Len returns the length of the serialized tx
func (pt *PushNote) Len() uint64 {
	return uint64(len(pt.Bytes()))
}

// ID returns the hash of the push note
func (pt *PushNote) ID() util.Hash {
	return util.BytesToHash(util.Blake2b256(pt.Bytes()))
}

// BytesAndID returns the serialized version of the tx and the id
func (pt *PushNote) BytesAndID() ([]byte, util.Hash) {
	bz := pt.Bytes()
	return bz, util.BytesToHash(bz)
}

// TxSize is the size of the transaction
func (pt *PushNote) TxSize() uint {
	return uint(len(pt.Bytes()))
}

// BillableSize is the size of the transaction + pushed objects
func (pt *PushNote) BillableSize() uint64 {
	return pt.LenMinusFee() + pt.Size
}

// GetSize returns the total pushed objects size
func (pt *PushNote) GetSize() uint64 {
	return pt.Size
}

// TotalFee returns the sum of reference update fees
func (pt *PushNote) TotalFee() util.String {
	sum := decimal.NewFromFloat(0)
	for _, r := range pt.References {
		sum = sum.Add(r.Fee.Decimal())
	}
	return util.String(sum.String())
}

// makePackfileFromPushNote creates a packfile from a PushNote
func makePackfileFromPushNote(repo types.BareRepo, tx *PushNote) (io.ReadSeeker, error) {

	var buf = bytes.NewBuffer(nil)
	enc := packfile.NewEncoder(buf, repo.GetStorer(), true)

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

// makeReferenceUpdateRequest creates a git reference update request from a push
// transaction. This is what git push sends to the git-receive-pack.
func makeReferenceUpdateRequest(repo types.BareRepo, tx *PushNote) (io.ReadSeeker, error) {

	// Generate a packfile
	packfile, err := makePackfileFromPushNote(repo, tx)
	if err != nil {
		return nil, err
	}

	caps := capability.NewList()
	caps.Add(capability.Agent, "git/2.x")
	caps.Add(capability.ReportStatus)
	caps.Add(capability.OFSDelta)
	caps.Add(capability.DeleteRefs)

	ru := packp.NewReferenceUpdateRequestFromCapabilities(caps)
	ru.Packfile = ioutil.NopCloser(packfile)
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
	repo types.BareRepo,
	oldState,
	newState types.BareRepoState) (*PushNote, error) {

	// Compute the changes between old and new states
	tx := &PushNote{References: []*types.PushedReference{}}
	changes := oldState.GetChanges(newState)

	// For each changed references, generate a PushedReference object
	for _, change := range changes.References.Changes {

		newHash := change.Item.GetData()
		var commit *object.Commit
		var err error
		var objHashes = []string{}

		// Get the hash of the old version of the changed reference
		var changedRefOld = oldState.GetReferences().Get(change.Item.GetName())
		var changedRefOldVerHash string
		if changedRefOld != nil {
			changedRefOldVerHash = changedRefOld.GetData()
		}

		// Get the commit object, if changed reference is a branch
		if isBranch(change.Item.GetName()) {
			commit, err = repo.CommitObject(plumbing.NewHash(newHash))
			if err != nil {
				return nil, err
			}
		}

		// Get the commit referenced by the tag
		if isTag(change.Item.GetName()) {
			nameParts := strings.Split(change.Item.GetName(), "/")

			// Get the tag from the repository.
			// If we can't find it and the change type is a 'remove', skip to
			// the reference addition section
			tag, err := repo.Tag(nameParts[len(nameParts)-1])
			if err != nil {
				if err == git.ErrTagNotFound && change.Action == types.ChangeTypeRemove {
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

				// Add the tag object as part of the objects updates
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
		if isNote(change.Item.GetName()) {
			commit, err = repo.CommitObject(plumbing.NewHash(newHash))
			if err != nil {
				return nil, err
			}
		}

	addRef:
		// Generate the pushed reference object depending on the type of change
		// that happened to the reference.
		switch change.Action {
		case types.ChangeTypeNew:
			histHashes, err := getCommitHistory(repo, commit, "")
			if err != nil {
				return nil, err
			}
			tx.References = append(tx.References, &types.PushedReference{
				Name:    change.Item.GetName(),
				NewHash: newHash,
				OldHash: plumbing.ZeroHash.String(),
				Objects: append(objHashes, histHashes...),
			})

		case types.ChangeTypeUpdate:
			histHashes, err := getCommitHistory(repo, commit, changedRefOldVerHash)
			if err != nil {
				return nil, err
			}
			tx.References = append(tx.References, &types.PushedReference{
				Name:    change.Item.GetName(),
				Objects: append(objHashes, histHashes...),
				NewHash: newHash,
				OldHash: oldState.GetReferences().Get(change.Item.GetName()).GetData(),
			})

		case types.ChangeTypeRemove:
			tx.References = append(tx.References, &types.PushedReference{
				Name:    change.Item.GetName(),
				NewHash: plumbing.ZeroHash.String(),
				OldHash: changedRefOldVerHash,
			})
		}
	}

	return tx, nil
}

// makePackfile creates a git reference update request packfile from state
// changes between old and new repository state.
func makePackfile(
	repo types.BareRepo,
	oldState,
	newState types.BareRepoState) (io.ReadSeeker, error) {

	pushNote, err := makePushNoteFromStateChange(repo, oldState, newState)
	if err != nil {
		return nil, err
	}

	return makeReferenceUpdateRequest(repo, pushNote)
}
