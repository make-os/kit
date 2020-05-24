package types

import (
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// PushNotice implements types.PushNotice
type PushNote struct {
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`

	// TargetRepo is the target repo local instance
	TargetRepo types.LocalRepo `json:",flatten,omitempty" msgpack:"-" mapstructure:"-"`

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

	// Size is the size of all objects pushed
	Size uint64 `json:"size,omitempty" msgpack:"size,omitempty"`

	// Timestamp is the unix timestamp
	Timestamp int64 `json:"timestamp,omitempty" msgpack:"timestamp,omitempty"`

	// PusherAcctNonce is the next nonce of the pusher's account
	PusherAcctNonce uint64 `json:"accountNonce,omitempty" msgpack:"accountNonce,omitempty"`

	// NodeSig is the signature of the node that created the PushNotice
	NodeSig []byte `json:"nodeSig,omitempty" msgpack:"nodeSig,omitempty"`

	// NodePubKey is the public key of the push note signer
	NodePubKey util.Bytes32 `json:"nodePubKey,omitempty" msgpack:"nodePubKey,omitempty"`

	// serialized caches the serialized bytes of object
	bytes []byte `msgpack:"-"`
}

// GetTargetRepo returns the target repository
func (pt *PushNote) GetTargetRepo() types.LocalRepo {
	return pt.TargetRepo
}

// GetTargetRepo returns the target repository
func (pt *PushNote) SetTargetRepo(repo types.LocalRepo) {
	pt.TargetRepo = repo
}

// GetNodePubKey returns the push node's public key
func (pt *PushNote) GetNodePubKey() util.Bytes32 {
	return pt.NodePubKey
}

// GetNodeSignature returns the push note signature
func (pt *PushNote) GetNodeSignature() []byte {
	return pt.NodeSig
}

// GetPusherKeyID returns the pusher pusher key ID
func (pt *PushNote) GetPusherKeyID() []byte {
	return pt.PushKeyID
}

// GetPusherAddress returns the pusher's address
func (pt *PushNote) GetPusherAddress() util.Address {
	return pt.PusherAddress
}

// GetPusherAccountNonce returns the pusher account nonce
func (pt *PushNote) GetPusherAccountNonce() uint64 {
	return pt.PusherAcctNonce
}

// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed with 0x
func (pt *PushNote) GetPusherKeyIDString() string {
	return crypto.BytesToPushKeyID(pt.PushKeyID)
}

// GetTimestamp returns the timestamp
func (pt *PushNote) GetTimestamp() int64 {
	return pt.Timestamp
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

// GetNamespace returns the target namespace
func (pt *PushNote) GetNamespace() string {
	return pt.Namespace
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

// GetValue returns the total value
func (pt *PushNote) GetValue() util.String {
	var value = decimal.Zero
	for _, ref := range pt.References {
		if !ref.Value.Empty() {
			value = value.Add(ref.Value.Decimal())
		}
	}
	return util.String(value.String())
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

// Pool represents a pool for holding and ordering git push transactions
type Pool interface {

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
	Add(tx PushNotice, noValidation ...bool) error

	// Full returns true if the pool is full
	Full() bool

	// RepoHasPushNote returns true if the given repo has a transaction in the pool
	RepoHasPushNote(repo string) bool

	// Get finds and returns a push note
	Get(noteID string) *PushNote

	// Len returns the number of items in the pool
	Len() int

	// Remove removes a push note
	Remove(pushNote PushNotice)
}

type PushNotice interface {
	GetTargetRepo() types.LocalRepo
	SetTargetRepo(repo types.LocalRepo)
	GetPusherKeyID() []byte
	GetPusherAddress() util.Address
	GetPusherAccountNonce() uint64
	GetPusherKeyIDString() string
	EncodeMsgpack(enc *msgpack.Encoder) error
	DecodeMsgpack(dec *msgpack.Decoder) error
	Bytes(recompute ...bool) []byte
	BytesNoCache() []byte
	BytesNoSig() []byte
	GetPushedObjects() []string
	GetEcoSize() uint64
	GetNodePubKey() util.Bytes32
	GetNodeSignature() []byte
	GetRepoName() string
	GetNamespace() string
	GetTimestamp() int64
	GetPushedReferences() PushedReferences
	Len() uint64
	ID(recompute ...bool) util.Bytes32
	BytesAndID(recompute ...bool) ([]byte, util.Bytes32)
	TxSize() uint
	BillableSize() uint64
	GetSize() uint64
	GetFee() util.String
	GetValue() util.String
}

// PushedReference represents a reference that was pushed by git client
type PushedReference struct {
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`
	Name                  string               `json:"name" msgpack:"name,omitempty"`       // The full name of the reference
	OldHash               string               `json:"oldHash" msgpack:"oldHash,omitempty"` // The hash of the reference before the push
	NewHash               string               `json:"newHash" msgpack:"newHash,omitempty"` // The hash of the reference after the push
	Nonce                 uint64               `json:"nonce" msgpack:"nonce,omitempty"`     // The next repo nonce of the reference
	Objects               []string             `json:"objects" msgpack:"objects,omitempty"` // A list of objects pushed to the reference
	MergeProposalID       string               `json:"mergeID" msgpack:"mergeID,omitempty"` // The merge proposal ID the reference is complaint with.
	Fee                   util.String          `json:"fee" msgpack:"fee,omitempty"`         // The network fee to pay for pushing the reference
	Value                 util.String          `json:"value" msgpack:"value,omitempty"`     // Additional fee to pay for special operation
	PushSig               []byte               `json:"pushSig" msgpack:"pushSig,omitempty"` // The signature of from the push request token
	Data                  *types.ReferenceData `json:"data" msgpack:"data,omitempty"`       // Contains updates to the reference data
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pr *PushedReference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		pr.Name,
		pr.OldHash,
		pr.NewHash,
		pr.Nonce,
		pr.Objects,
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
		&pr.Objects,
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
