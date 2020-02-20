package core

import (
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// PushNote implements types.PushNote
type PushNote struct {
	util.DecoderHelper `json:",flatten" msgpack:"-" mapstructure:"-"`
	TargetRepo         BareRepo         `json:",flatten" msgpack:"-" mapstructure:"-"`
	RepoName           string           `json:"repoName" msgpack:"repoName"`         // The name of the repo
	References         PushedReferences `json:"references" msgpack:"references"`     // A list of references pushed
	PusherKeyID        []byte           `json:"pusherKeyId" msgpack:"pusherKeyId"`   // The PGP key of the pusher
	PusherAddress      util.String      `json:"pusherAddr" msgpack:"pusherAddr"`     // The Address of the pusher
	Size               uint64           `json:"size" msgpack:"size"`                 // Total size of all objects pushed
	Timestamp          int64            `json:"timestamp" msgpack:"timestamp"`       // Unix timestamp
	AccountNonce       uint64           `json:"accountNonce" msgpack:"accountNonce"` // Next nonce of the pusher's account
	Fee                util.String      `json:"fee" msgpack:"fee"`                   // Total fees to pay for the pushed references
	NodeSig            []byte           `json:"nodeSig" msgpack:"nodeSig"`           // The signature of the node that created the PushNote
	NodePubKey         util.Bytes32     `json:"nodePubKey" msgpack:"nodePubKey"`     // The public key of the push note signer
}

// GetTargetRepo returns the target repository
func (pt *PushNote) GetTargetRepo() BareRepo {
	return pt.TargetRepo
}

// GetPusherKeyID returns the pusher gpg key ID
func (pt *PushNote) GetPusherKeyID() []byte {
	return pt.PusherKeyID
}

// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed
// with 0x
func (pt *PushNote) GetPusherKeyIDString() string {
	return util.MustToRSAPubKeyID(pt.PusherKeyID)
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pt *PushNote) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		pt.RepoName,
		pt.References,
		pt.PusherKeyID,
		pt.PusherAddress,
		pt.Size,
		pt.Timestamp,
		pt.AccountNonce,
		pt.Fee,
		pt.NodeSig,
		pt.NodePubKey)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pt *PushNote) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pt.DecodeMulti(dec,
		&pt.RepoName,
		&pt.References,
		&pt.PusherKeyID,
		&pt.PusherAddress,
		&pt.Size,
		&pt.Timestamp,
		&pt.AccountNonce,
		&pt.Fee,
		&pt.NodeSig,
		&pt.NodePubKey)
}

// Bytes returns a serialized version of the object
func (pt *PushNote) Bytes() []byte {
	return util.ObjectToBytes(pt)
}

// BytesNoSig returns a serialized version of the object without the signature
func (pt *PushNote) BytesNoSig() []byte {
	sig := pt.NodeSig
	pt.NodeSig = nil
	bz := pt.Bytes()
	pt.NodeSig = sig
	return bz
}

// GetPushedObjects returns all objects from all pushed references without a
// delete directive.
// ignoreDelRefs cause deleted references' objects to not be include in the result
func (pt *PushNote) GetPushedObjects(ignoreDelRefs bool) (objs []string) {
	for _, ref := range pt.GetPushedReferences() {
		if ignoreDelRefs && ref.Delete {
			continue
		}
		objs = append(objs, ref.Objects...)
	}
	return
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

// GetFee returns the push fee
func (pt *PushNote) GetFee() util.String {
	if pt.Fee.Empty() {
		return util.ZeroString
	}
	return pt.Fee
}

// ReferenceHash describes the current and previous state hash of a reference
type ReferenceHash struct {
	Hash util.Bytes32 `json:"hash" msgpack:"hash" mapstructure:"hash"`
}

// ReferenceHashes is a collection of ReferenceHash
type ReferenceHashes []*ReferenceHash

// ID returns the id of the collection
func (r *ReferenceHashes) ID() util.Bytes32 {
	bz := util.ObjectToBytes(r)
	return util.BytesToBytes32(util.Blake2b256(bz))
}

// PushOK is used to endorse a push note
type PushOK struct {
	util.DecoderHelper `json:",flatten" msgpack:"-" mapstructure:"-"`
	PushNoteID         util.Bytes32    `json:"pushNoteID" msgpack:"pushNoteID" mapstructure:"pushNoteID"`
	ReferencesHash     ReferenceHashes `json:"refsHash" msgpack:"refsHash" mapstructure:"refsHash"`
	SenderPubKey       util.Bytes32    `json:"senderPubKey" msgpack:"senderPubKey" mapstructure:"senderPubKey"`
	Sig                util.Bytes64    `json:"sig" msgpack:"sig" mapstructure:"sig"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (po *PushOK) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(po.PushNoteID, po.ReferencesHash, po.SenderPubKey, po.Sig)
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
	return util.ObjectToBytes(po)
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

