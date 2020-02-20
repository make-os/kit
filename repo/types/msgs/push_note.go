package msgs

import (
	"gitlab.com/makeos/mosdef/repo/types/core"
	"gitlab.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// PushNote implements types.PushNote
type PushNote struct {
	util.DecoderHelper `json:",flatten" msgpack:"-" mapstructure:"-"`
	TargetRepo         core.BareRepo         `json:",flatten" msgpack:"-" mapstructure:"-"`
	RepoName           string                `json:"repoName" msgpack:"repoName"`         // The name of the repo
	References         core.PushedReferences `json:"references" msgpack:"references"`     // A list of references pushed
	PusherKeyID        []byte                `json:"pusherKeyId" msgpack:"pusherKeyId"`   // The PGP key of the pusher
	PusherAddress      util.String           `json:"pusherAddr" msgpack:"pusherAddr"`     // The Address of the pusher
	Size               uint64                `json:"size" msgpack:"size"`                 // Total size of all objects pushed
	Timestamp          int64                 `json:"timestamp" msgpack:"timestamp"`       // Unix timestamp
	AccountNonce       uint64                `json:"accountNonce" msgpack:"accountNonce"` // Next nonce of the pusher's account
	Fee                util.String           `json:"fee" msgpack:"fee"`                   // Total fees to pay for the pushed references
	NodeSig            []byte                `json:"nodeSig" msgpack:"nodeSig"`           // The signature of the node that created the PushNote
	NodePubKey         util.Bytes32          `json:"nodePubKey" msgpack:"nodePubKey"`     // The public key of the push note signer
}

// GetTargetRepo returns the target repository
func (pt *PushNote) GetTargetRepo() core.BareRepo {
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
func (pt *PushNote) GetPushedReferences() core.PushedReferences {
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

