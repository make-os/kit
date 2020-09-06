package types

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/util"
	crypto2 "github.com/make-os/lobe/util/crypto"
	"github.com/make-os/lobe/util/identifier"
	"github.com/shopspring/decimal"
	"github.com/vmihailenco/msgpack"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// PushNote implements types.PushNote
type Note struct {
	util.CodecUtil `json:"-" msgpack:"-" mapstructure:"-"`

	// TargetRepo is the target repo local instance
	TargetRepo types.LocalRepo `json:",flatten,omitempty" msgpack:"-" mapstructure:"-"`

	// RepoName is the name of the repo. If Namespace is set, it will be the
	// domain pointing to the actual repository.
	RepoName string `json:"repo,omitempty" msgpack:"repo,omitempty"`

	// Namespace is the namespace which the pusher is targeting.
	Namespace string `json:"namespace,omitempty" msgpack:"namespace,omitempty"`

	// References contains all references pushed
	References PushedReferences `json:"references,omitempty" msgpack:"references,omitempty"`

	// PushKeyID is the push key ID of the pusher
	PushKeyID util.Bytes `json:"pusherKeyId,omitempty" msgpack:"pusherKeyId,omitempty"`

	// PusherAddress is the Address of the pusher
	PusherAddress identifier.Address `json:"pusherAddr,omitempty" msgpack:"pusherAddr,omitempty"`

	// Size is the size of all objects pushed
	Size uint64 `json:"size,omitempty" msgpack:"size,omitempty"`

	// Timestamp is the unix timestamp
	Timestamp int64 `json:"timestamp,omitempty" msgpack:"timestamp,omitempty"`

	// PusherAcctNonce is the next nonce of the pusher's account
	PusherAcctNonce uint64 `json:"accountNonce,omitempty" msgpack:"accountNonce,omitempty"`

	// RemoteNodeSig is the signature of the note creator
	RemoteNodeSig util.Bytes `json:"creatorSig,omitempty" msgpack:"creatorSig,omitempty"`

	// RemotePubKey is the public key of the note creator/signer
	CreatorPubKey util.Bytes32 `json:"creatorPubKey,omitempty" msgpack:"creatorPubKey,omitempty"`

	// serialized caches the serialized bytes of object
	bytes []byte

	// FromPeer indicates that the note was received from a remote
	// peer and not created by the local node
	FromRemotePeer bool `json:"-" msgpack:"-"`
}

// GetTargetRepo returns the target repository
func (pt *Note) GetTargetRepo() types.LocalRepo {
	return pt.TargetRepo
}

// GetTargetRepo returns the target repository
func (pt *Note) SetTargetRepo(repo types.LocalRepo) {
	pt.TargetRepo = repo
}

// GetCreatorPubKey returns the note creator's public key
func (pt *Note) GetCreatorPubKey() util.Bytes32 {
	return pt.CreatorPubKey
}

// GetNodeSignature returns the push note signature
func (pt *Note) GetNodeSignature() []byte {
	return pt.RemoteNodeSig
}

// GetPusherKeyID returns the pusher pusher key ID
func (pt *Note) GetPusherKeyID() []byte {
	return pt.PushKeyID
}

// GetPusherAddress returns the pusher's address
func (pt *Note) GetPusherAddress() identifier.Address {
	return pt.PusherAddress
}

// GetPusherAccountNonce returns the pusher account nonce
func (pt *Note) GetPusherAccountNonce() uint64 {
	return pt.PusherAcctNonce
}

// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed with 0x
func (pt *Note) GetPusherKeyIDString() string {
	return crypto.BytesToPushKeyID(pt.PushKeyID)
}

// GetTimestamp returns the timestamp
func (pt *Note) GetTimestamp() int64 {
	return pt.Timestamp
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pt *Note) EncodeMsgpack(enc *msgpack.Encoder) error {
	return pt.EncodeMulti(enc,
		pt.RepoName,
		pt.Namespace,
		pt.References,
		pt.PushKeyID,
		pt.PusherAddress,
		pt.Size,
		pt.Timestamp,
		pt.PusherAcctNonce,
		pt.RemoteNodeSig,
		pt.CreatorPubKey)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pt *Note) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pt.DecodeMulti(dec,
		&pt.RepoName,
		&pt.Namespace,
		&pt.References,
		&pt.PushKeyID,
		&pt.PusherAddress,
		&pt.Size,
		&pt.Timestamp,
		&pt.PusherAcctNonce,
		&pt.RemoteNodeSig,
		&pt.CreatorPubKey)
}

// Bytes returns a serialized version of the object. If this function was previously called,
// the cached output from the previous call is returned instead of re-serializing the object.
// Set recompute to true to force re-serialization.
func (pt *Note) Bytes(recompute ...bool) []byte {
	if (len(recompute) > 0 && recompute[0]) || len(pt.bytes) == 0 {
		pt.bytes = pt.BytesNoCache()
		return pt.bytes
	}
	return pt.bytes
}

// BytesNoCache returns the serialized version of the object but does not cache it.
func (pt *Note) BytesNoCache() []byte {
	return util.ToBytes(pt)
}

// BytesNoSig returns a serialized version of the object without the signature
func (pt *Note) BytesNoSig() []byte {
	sig := pt.RemoteNodeSig
	pt.RemoteNodeSig = nil
	bz := pt.BytesNoCache()
	pt.RemoteNodeSig = sig
	return bz
}

// GetEcoSize returns a size of the push note used for economics calculation.
func (pt *Note) GetEcoSize() uint64 {
	size := len(pt.Bytes())
	return uint64(size)
}

// GetRepoName returns the name of the repo receiving the push
func (pt *Note) GetRepoName() string {
	return pt.RepoName
}

// GetNamespace returns the target namespace
func (pt *Note) GetNamespace() string {
	return pt.Namespace
}

// GetPushedReferences returns the pushed references
func (pt *Note) GetPushedReferences() PushedReferences {
	return pt.References
}

// Len returns the length of the serialized tx
func (pt *Note) Len() uint64 {
	return uint64(len(pt.Bytes()))
}

// ID returns the hash of the push note
func (pt *Note) ID(recompute ...bool) util.Bytes32 {
	return util.BytesToBytes32(crypto2.Blake2b256(pt.Bytes(recompute...)))
}

// IsFromRemotePeer checks whether the note was sent by a remote peer
func (pt *Note) IsFromRemotePeer() bool {
	return pt.FromRemotePeer
}

// BytesAndID returns the serialized version of the tx and the id
func (pt *Note) BytesAndID(recompute ...bool) ([]byte, util.Bytes32) {
	bz := pt.Bytes(recompute...)
	return bz, util.BytesToBytes32(crypto2.Blake2b256(bz))
}

// TxSize is the size of the transaction
func (pt *Note) TxSize() uint {
	return uint(len(pt.Bytes()))
}

// SizeForFeeCal is the size of the transaction + pushed objects
func (pt *Note) SizeForFeeCal() uint64 {
	return pt.GetEcoSize() + pt.Size
}

// GetSize returns the total pushed objects size
func (pt *Note) GetSize() uint64 {
	return pt.Size
}

// GetFee returns the total push fee
func (pt *Note) GetFee() util.String {
	var fee = decimal.Zero
	for _, ref := range pt.References {
		if !ref.Fee.Empty() {
			fee = fee.Add(ref.Fee.Decimal())
		}
	}
	return util.String(fee.String())
}

// GetValue returns the total value
func (pt *Note) GetValue() util.String {
	var value = decimal.Zero
	for _, ref := range pt.References {
		if !ref.Value.Empty() {
			value = value.Add(ref.Value.Decimal())
		}
	}
	return util.String(value.String())
}

// ToMap returns the map equivalent of the note
func (pt *Note) ToMap() map[string]interface{} {
	pt.TargetRepo = nil
	return util.ToBasicMap(pt)
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
	return util.BytesToBytes32(crypto2.Blake2b256(bz))
}

// Endorsement represents a push endorsement
type Endorsement interface {
	// ID returns the hash of the object
	ID() util.Bytes32
	// Bytes returns a serialized version of the object
	Bytes() []byte
	// BytesAndID returns the serialized version of the tx and the id
	BytesAndID() ([]byte, util.Bytes32)
}

// Endorsement is used to endorse a push note
type PushEndorsement struct {
	util.CodecUtil `json:"-" msgpack:"-" mapstructure:"-"`

	// NoteID is the ID of the push note to be endorsed.
	NoteID util.Bytes `json:"noteID" msgpack:"noteID,omitempty" mapstructure:"noteID"`

	// References contains the current hash of the push references
	References EndorsedReferences `json:"refs" msgpack:"refs,omitempty" mapstructure:"refs"`

	// EndorserPubKey is the public key of the endorser
	EndorserPubKey util.Bytes32 `json:"pubKey" msgpack:"pubKey,omitempty" mapstructure:"pubKey"`

	// SigBLS is a 64 bytes BLS signature created using the BLS key of the endorser.
	SigBLS []byte `json:"sigBLS" msgpack:"sigBLS,omitempty" mapstructure:"sigBLS"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (e *PushEndorsement) EncodeMsgpack(enc *msgpack.Encoder) error {
	return e.EncodeMulti(enc,
		e.NoteID,
		e.References,
		e.EndorserPubKey.Bytes(),
		e.SigBLS)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (e *PushEndorsement) DecodeMsgpack(dec *msgpack.Decoder) error {
	err := e.DecodeMulti(dec,
		&e.NoteID,
		&e.References,
		&e.EndorserPubKey,
		&e.SigBLS)
	if err != nil {
		return err
	}
	return nil
}

// ID returns the hash of the object
func (e *PushEndorsement) ID() util.Bytes32 {
	return util.BytesToBytes32(crypto2.Blake2b256(e.Bytes()))
}

// Bytes returns a serialized version of the object
func (e *PushEndorsement) Bytes() []byte {
	return util.ToBytes(e)
}

// BytesForBLSSig returns the serialized version of the endorsement for creating BLS signature.
// It will not include the following fields: SigBLS and EndorserPubKey.
func (e *PushEndorsement) BytesForBLSSig() []byte {
	sigBLS, spk := e.SigBLS, e.EndorserPubKey
	e.SigBLS, e.EndorserPubKey = nil, util.EmptyBytes32
	msg := e.Bytes()
	e.SigBLS, e.EndorserPubKey = sigBLS, spk
	return msg
}

// BytesForSig returns the serialized version of the endorsement for creating BLS signature.
// It will not include the following fields: SigBLS
func (e *PushEndorsement) BytesForSig() []byte {
	sigBLS := e.SigBLS
	e.SigBLS = nil
	msg := e.Bytes()
	e.SigBLS = sigBLS
	return msg
}

// BytesAndID returns the serialized version of the tx and the id
func (e *PushEndorsement) BytesAndID() ([]byte, util.Bytes32) {
	bz := e.Bytes()
	return bz, util.BytesToBytes32(crypto2.Blake2b256(bz))
}

// Clone clones the object
func (e *PushEndorsement) Clone() *PushEndorsement {
	cp := &PushEndorsement{}
	cp.NoteID = e.NoteID
	cp.EndorserPubKey = util.BytesToBytes32(e.EndorserPubKey.Bytes())
	cp.SigBLS = e.SigBLS
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

// PushPool represents a pool for ordering git push transactions
type PushPool interface {

	// Register a push transaction to the pool.
	//
	// Check all the references to ensure there are no identical (same repo,
	// reference and nonce) references with same nonce in the pool. A valid
	// reference is one which has no identical reference with a higher fee rate in
	// the pool. If an identical reference exist in the pool with an inferior fee
	// rate, the existing tx holding the reference is eligible for replacable by tx
	// holding the reference with a superior fee rate. In cases where more than one
	// reference of tx is superior to multiple references in multiple transactions,
	// replacement will only happen if the fee rate of tx is higher than the
	// combined fee rate of the replaceable transactions.
	//
	// noValidation disables tx validation
	Add(note PushNote) error

	// Full returns true if the pool is full
	Full() bool

	// Get finds and returns a push note
	Get(noteID string) *Note

	// Len returns the number of items in the pool
	Len() int

	// Remove removes a push note
	Remove(pushNote PushNote)

	// HasSeen checks whether a note with the given ID was recently added
	HasSeen(noteID string) bool
}

type PushNote interface {
	GetTargetRepo() types.LocalRepo
	SetTargetRepo(repo types.LocalRepo)
	GetPusherKeyID() []byte
	GetPusherAddress() identifier.Address
	GetPusherAccountNonce() uint64
	GetPusherKeyIDString() string
	EncodeMsgpack(enc *msgpack.Encoder) error
	DecodeMsgpack(dec *msgpack.Decoder) error
	Bytes(recompute ...bool) []byte
	BytesNoCache() []byte
	BytesNoSig() []byte
	GetEcoSize() uint64
	GetCreatorPubKey() util.Bytes32
	GetNodeSignature() []byte
	GetRepoName() string
	GetNamespace() string
	GetTimestamp() int64
	GetPushedReferences() PushedReferences
	Len() uint64
	ID(recompute ...bool) util.Bytes32
	BytesAndID(recompute ...bool) ([]byte, util.Bytes32)
	TxSize() uint
	SizeForFeeCal() uint64
	GetSize() uint64
	GetFee() util.String
	GetValue() util.String
	IsFromRemotePeer() bool
}

// PushedReference represents a reference that was pushed by git client
type PushedReference struct {
	util.CodecUtil  `json:"-" msgpack:"-" mapstructure:"-"`
	Name            string               `json:"name,omitempty" msgpack:"name,omitempty"`       // The full name of the reference
	OldHash         string               `json:"oldHash,omitempty" msgpack:"oldHash,omitempty"` // The hash of the reference before the push
	NewHash         string               `json:"newHash,omitempty" msgpack:"newHash,omitempty"` // The hash of the reference after the push
	Nonce           uint64               `json:"nonce,omitempty" msgpack:"nonce,omitempty"`     // The next repo nonce of the reference
	MergeProposalID string               `json:"mergeID,omitempty" msgpack:"mergeID,omitempty"` // The merge proposal ID the reference is complaint with.
	Fee             util.String          `json:"fee,omitempty" msgpack:"fee,omitempty"`         // The network fee to pay for pushing the reference
	Value           util.String          `json:"value,omitempty" msgpack:"value,omitempty"`     // Additional fee to pay for special operation
	PushSig         util.Bytes           `json:"pushSig,omitempty" msgpack:"pushSig,omitempty"` // The signature of from the push request token
	Data            *types.ReferenceData `json:"data,omitempty" msgpack:"data,omitempty"`       // Contains updates to the reference data
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pr *PushedReference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return pr.EncodeMulti(enc,
		pr.Name,
		pr.OldHash,
		pr.NewHash,
		pr.Nonce,
		pr.MergeProposalID,
		pr.Fee,
		pr.Value,
		pr.Data,
		pr.PushSig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pr *PushedReference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pr.DecodeMulti(dec,
		&pr.Name,
		&pr.OldHash,
		&pr.NewHash,
		&pr.Nonce,
		&pr.MergeProposalID,
		&pr.Fee,
		&pr.Value,
		&pr.Data,
		&pr.PushSig)
}

// IsDeletable checks whether the pushed reference can be deleted
func (pr *PushedReference) IsDeletable() bool {
	return pr.NewHash == plumbing.ZeroHash.String()
}

// PushedReferences represents a collection of pushed references
type PushedReferences []*PushedReference

// GetByName finds a pushed reference by name
func (pf *PushedReferences) GetByName(name string) *PushedReference {
	for _, r := range *pf {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// Names returns the names of the references
func (pf *PushedReferences) Names() (names []string) {
	for _, r := range *pf {
		names = append(names, r.Name)
	}
	return
}
