package core

import (
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// PushNote implements types.PushNote
type PushNote struct {
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`

	// TargetRepo is the target repo local instance
	TargetRepo LocalRepo `json:",flatten,omitempty" msgpack:"-" mapstructure:"-"`

	// RepoName is the name of the repo
	RepoName string `json:"repo,omitempty" msgpack:"repo,omitempty"`

	// Namespace is the namespace which the pusher is targeting.
	Namespace string `json:"namespace,omitempty" msgpack:"namespace,omitempty"`

	// References contains all references pushed
	References PushedReferences `json:"references,omitempty" msgpack:"references,omitempty"`

	// PushKeyID is the push key ID of the pusher
	PushKeyID []byte `json:"pusherKeyId,omitempty" msgpack:"pusherKeyId,omitempty"`

	// PusherAddress is the Address of the pusher
	PusherAddress util.Address `json:"pusherAddr,omitempty" msgpack:"pusherAddr,omitempty"`

	// Size is thet otal size of all objects pushed
	Size uint64 `json:"size,omitempty" msgpack:"size,omitempty"`

	// Timestamp is the unix timestamp
	Timestamp int64 `json:"timestamp,omitempty" msgpack:"timestamp,omitempty"`

	// PusherAcctNonce is the next nonce of the pusher's account
	PusherAcctNonce uint64 `json:"accountNonce,omitempty" msgpack:"accountNonce,omitempty"`

	// NodeSig is the signature of the node that created the PushNote
	NodeSig []byte `json:"nodeSig,omitempty" msgpack:"nodeSig,omitempty"`

	// NodePubKey is the public key of the push note signer
	NodePubKey util.Bytes32 `json:"nodePubKey,omitempty" msgpack:"nodePubKey,omitempty"`

	// serialized caches the serialized bytes of object
	bytes []byte `msgpack:"-"`
}

// GetTargetRepo returns the target repository
func (pt *PushNote) GetTargetRepo() LocalRepo {
	return pt.TargetRepo
}

// GetPusherKeyID returns the pusher gpg key ID
func (pt *PushNote) GetPusherKeyID() []byte {
	return pt.PushKeyID
}

// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed with 0x
func (pt *PushNote) GetPusherKeyIDString() string {
	return crypto.BytesToPushKeyID(pt.PushKeyID)
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pt *PushNote) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		pt.RepoName,
		pt.Namespace,
		pt.References,
		pt.PushKeyID,
		pt.PusherAddress,
		pt.Size,
		pt.Timestamp,
		pt.PusherAcctNonce,
		pt.NodeSig,
		pt.NodePubKey)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pt *PushNote) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pt.DecodeMulti(dec,
		&pt.RepoName,
		&pt.Namespace,
		&pt.References,
		&pt.PushKeyID,
		&pt.PusherAddress,
		&pt.Size,
		&pt.Timestamp,
		&pt.PusherAcctNonce,
		&pt.NodeSig,
		&pt.NodePubKey)
}

// Bytes returns a serialized version of the object. If this function was previously called,
// the cached output from the previous call is returned instead of re-serializing the object.
// Set recompute to true to force re-serialization.
func (pt *PushNote) Bytes(recompute ...bool) []byte {
	if (len(recompute) > 0 && recompute[0]) || len(pt.bytes) == 0 {
		pt.bytes = pt.BytesNoCache()
		return pt.bytes
	}
	return pt.bytes
}

// BytesNoCache returns the serialized version of the object but does not cache it.
func (pt *PushNote) BytesNoCache() []byte {
	return util.ToBytes(pt)
}

// BytesNoSig returns a serialized version of the object without the signature
func (pt *PushNote) BytesNoSig() []byte {
	sig := pt.NodeSig
	pt.NodeSig = nil
	bz := pt.BytesNoCache()
	pt.NodeSig = sig
	return bz
}

// GetPushedObjects returns all objects from non-deleted pushed references.
// ignoreDelRefs cause deleted references' objects to not be include in the result
func (pt *PushNote) GetPushedObjects() []string {
	objs := make(map[string]struct{})
	for _, ref := range pt.GetPushedReferences() {
		for _, obj := range ref.Objects {
			objs[obj] = struct{}{}
		}
	}
	return funk.Keys(objs).([]string)
}

// GetEcoSize returns a size of the push note used for economics calculation.
func (pt *PushNote) GetEcoSize() uint64 {
	size := len(pt.Bytes())
	return uint64(size)
}

// GetRepoName returns the name of the repo receiving the push
func (pt *PushNote) GetRepoName() string {
	return pt.RepoName
}

// GetPushedReferences returns the pushed references
func (pt *PushNote) GetPushedReferences() PushedReferences {
	return pt.References
}

// Len returns the length of the serialized tx
func (pt *PushNote) Len() uint64 {
	return uint64(len(pt.Bytes()))
}

// ID returns the hash of the push note
func (pt *PushNote) ID(recompute ...bool) util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(pt.Bytes(recompute...)))
}

// BytesAndID returns the serialized version of the tx and the id
func (pt *PushNote) BytesAndID(recompute ...bool) ([]byte, util.Bytes32) {
	bz := pt.Bytes(recompute...)
	return bz, util.BytesToBytes32(util.Blake2b256(bz))
}

// TxSize is the size of the transaction
func (pt *PushNote) TxSize() uint {
	return uint(len(pt.Bytes()))
}

// BillableSize is the size of the transaction + pushed objects
func (pt *PushNote) BillableSize() uint64 {
	return pt.GetEcoSize() + pt.Size
}

// GetSize returns the total pushed objects size
func (pt *PushNote) GetSize() uint64 {
	return pt.Size
}

// GetFee returns the total push fee
func (pt *PushNote) GetFee() util.String {
	var fee = decimal.Zero
	for _, ref := range pt.References {
		if !ref.Fee.Empty() {
			fee = fee.Add(ref.Fee.Decimal())
		}
	}
	return util.String(fee.String())
}

// EndorsedReference describes the current state of a reference endorsed by a host
type EndorsedReference struct {
	Hash []byte `json:"hash" msgpack:"hash,omitempty" mapstructure:"hash"`
}

// EndorsedReferences is a collection of EndorsedReference
type EndorsedReferences []*EndorsedReference

// ID returns the id of the collection
func (r *EndorsedReferences) ID() util.Bytes32 {
	bz := util.ToBytes(r)
	return util.BytesToBytes32(util.Blake2b256(bz))
}

// PushEndorsement is used to endorse a push note
type PushEndorsement struct {
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`
	NoteID                util.Bytes32       `json:"noteID" msgpack:"noteID,omitempty" mapstructure:"noteID"`
	References            EndorsedReferences `json:"endorsedRefs" msgpack:"endorsedRefs,omitempty" mapstructure:"endorsedRefs"`
	EndorserPubKey        util.Bytes32       `json:"endorser" msgpack:"endorser,omitempty" mapstructure:"endorser"`
	Sig                   util.Bytes64       `json:"sig" msgpack:"sig,omitempty" mapstructure:"sig"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (e *PushEndorsement) EncodeMsgpack(enc *msgpack.Encoder) error {
	return e.EncodeMulti(enc, e.NoteID, e.References, e.EndorserPubKey.Bytes(), e.Sig.Bytes())
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (e *PushEndorsement) DecodeMsgpack(dec *msgpack.Decoder) error {
	return e.DecodeMulti(dec, &e.NoteID, &e.References, &e.EndorserPubKey, &e.Sig)
}

// ID returns the hash of the object
func (e *PushEndorsement) ID() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(e.Bytes()))
}

// Bytes returns a serialized version of the object
func (e *PushEndorsement) Bytes() []byte {
	return util.ToBytes(e)
}

// BytesNoSig returns the serialized version of
func (e *PushEndorsement) BytesNoSig() []byte {
	sig := e.Sig
	e.Sig = util.EmptyBytes64
	msg := e.Bytes()
	e.Sig = sig
	return msg
}

// BytesNoSigAndSenderPubKey returns the serialized version of
func (e *PushEndorsement) BytesNoSigAndSenderPubKey() []byte {
	sig, spk := e.Sig, e.EndorserPubKey
	e.Sig = util.EmptyBytes64
	e.EndorserPubKey = util.EmptyBytes32
	msg := e.Bytes()
	e.Sig, e.EndorserPubKey = sig, spk
	return msg
}

// BytesAndID returns the serialized version of the tx and the id
func (e *PushEndorsement) BytesAndID() ([]byte, util.Bytes32) {
	bz := e.Bytes()
	return bz, util.BytesToBytes32(util.Blake2b256(bz))
}

// Clone clones the object
func (e *PushEndorsement) Clone() *PushEndorsement {
	cp := &PushEndorsement{}
	cp.NoteID = e.NoteID
	cp.EndorserPubKey = util.BytesToBytes32(e.EndorserPubKey.Bytes())
	cp.Sig = util.BytesToBytes64(e.Sig.Bytes())
	cp.References = []*EndorsedReference{}
	for _, rh := range e.References {
		cpEndorsement := &EndorsedReference{}
		cpHash := make([]byte, len(rh.Hash))
		copy(cpHash, rh.Hash)
		cpEndorsement.Hash = cpHash
		cp.References = append(cp.References, cpEndorsement)
	}
	return cp
}
