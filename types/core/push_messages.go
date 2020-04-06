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
	TargetRepo            BareRepo         `json:",flatten,omitempty" msgpack:"-" mapstructure:"-"`
	RepoName              string           `json:"repoName,omitempty" msgpack:"repoName,omitempty"`         // The name of the repo
	Namespace             string           `json:"namespace,omitempty" msgpack:"namespace,omitempty"`       // The namespace which the repo is under.
	References            PushedReferences `json:"references,omitempty" msgpack:"references,omitempty"`     // A list of references pushed
	PushKeyID             []byte           `json:"pusherKeyId,omitempty" msgpack:"pusherKeyId,omitempty"`   // The PGP key of the pusher
	PusherAddress         util.Address     `json:"pusherAddr,omitempty" msgpack:"pusherAddr,omitempty"`     // The Address of the pusher
	Size                  uint64           `json:"size,omitempty" msgpack:"size,omitempty"`                 // Total size of all objects pushed
	Timestamp             int64            `json:"timestamp,omitempty" msgpack:"timestamp,omitempty"`       // Unix timestamp
	PusherAcctNonce       uint64           `json:"accountNonce,omitempty" msgpack:"accountNonce,omitempty"` // Next nonce of the pusher's account
	NodeSig               []byte           `json:"nodeSig,omitempty" msgpack:"nodeSig,omitempty"`           // The signature of the node that created the PushNote
	NodePubKey            util.Bytes32     `json:"nodePubKey,omitempty" msgpack:"nodePubKey,omitempty"`     // The public key of the push note signer
}

// GetTargetRepo returns the target repository
func (pt *PushNote) GetTargetRepo() BareRepo {
	return pt.TargetRepo
}

// GetPusherKeyID returns the pusher gpg key ID
func (pt *PushNote) GetPusherKeyID() []byte {
	return pt.PushKeyID
}

// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed
// with 0x
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

// Bytes returns a serialized version of the object
func (pt *PushNote) Bytes() []byte {
	return util.ToBytes(pt)
}

// BytesNoSig returns a serialized version of the object without the signature
func (pt *PushNote) BytesNoSig() []byte {
	sig := pt.NodeSig
	pt.NodeSig = nil
	bz := pt.Bytes()
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
func (pt *PushNote) ID() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(pt.Bytes()))
}

// BytesAndID returns the serialized version of the tx and the id
func (pt *PushNote) BytesAndID() ([]byte, util.Bytes32) {
	bz := pt.Bytes()
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

// ReferenceHash describes the current and previous state hash of a reference
type ReferenceHash struct {
	Hash util.Bytes32 `json:"hash" msgpack:"hash" mapstructure:"hash"`
}

// ReferenceHashes is a collection of ReferenceHash
type ReferenceHashes []*ReferenceHash

// ID returns the id of the collection
func (r *ReferenceHashes) ID() util.Bytes32 {
	bz := util.ToBytes(r)
	return util.BytesToBytes32(util.Blake2b256(bz))
}

// PushOK is used to endorse a push note
type PushOK struct {
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`
	PushNoteID            util.Bytes32    `json:"pushNoteID" msgpack:"pushNoteID,omitempty" mapstructure:"pushNoteID"`
	ReferencesHash        ReferenceHashes `json:"refsHash" msgpack:"refsHash,omitempty" mapstructure:"refsHash"`
	SenderPubKey          util.Bytes32    `json:"senderPubKey" msgpack:"senderPubKey,omitempty" mapstructure:"senderPubKey"`
	Sig                   util.Bytes64    `json:"sig" msgpack:"sig,omitempty" mapstructure:"sig"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (po *PushOK) EncodeMsgpack(enc *msgpack.Encoder) error {
	return po.EncodeMulti(enc, po.PushNoteID, po.ReferencesHash, po.SenderPubKey.Bytes(), po.Sig.Bytes())
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (po *PushOK) DecodeMsgpack(dec *msgpack.Decoder) error {
	return po.DecodeMulti(dec, &po.PushNoteID, &po.ReferencesHash, &po.SenderPubKey, &po.Sig)
}

// ID returns the hash of the object
func (po *PushOK) ID() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(po.Bytes()))
}

// Bytes returns a serialized version of the object
func (po *PushOK) Bytes() []byte {
	return util.ToBytes(po)
}

// BytesNoSig returns the serialized version of
func (po *PushOK) BytesNoSig() []byte {
	sig := po.Sig
	po.Sig = util.EmptyBytes64
	msg := po.Bytes()
	po.Sig = sig
	return msg
}

// BytesNoSigAndSenderPubKey returns the serialized version of
func (po *PushOK) BytesNoSigAndSenderPubKey() []byte {
	sig, spk := po.Sig, po.SenderPubKey
	po.Sig = util.EmptyBytes64
	po.SenderPubKey = util.EmptyBytes32
	msg := po.Bytes()
	po.Sig, po.SenderPubKey = sig, spk
	return msg
}

// BytesAndID returns the serialized version of the tx and the id
func (po *PushOK) BytesAndID() ([]byte, util.Bytes32) {
	bz := po.Bytes()
	return bz, util.BytesToBytes32(util.Blake2b256(bz))
}

// Clone clones the object
func (po *PushOK) Clone() *PushOK {
	cp := &PushOK{}
	cp.PushNoteID = po.PushNoteID
	cp.SenderPubKey = util.BytesToBytes32(po.SenderPubKey.Bytes())
	cp.Sig = util.BytesToBytes64(po.Sig.Bytes())
	cp.ReferencesHash = []*ReferenceHash{}
	for _, rh := range po.ReferencesHash {
		cpRefHash := &ReferenceHash{}
		cpRefHash.Hash = util.BytesToBytes32(rh.Hash.Bytes())
		cp.ReferencesHash = append(cp.ReferencesHash, cpRefHash)
	}
	return cp
}
