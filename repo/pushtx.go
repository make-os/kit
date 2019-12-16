package repo

import (
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
	"github.com/vmihailenco/msgpack"
)

// PushTx implements types.PushTx
type PushTx struct {
	targetRepo  types.BareRepo
	RepoName    string                 `json:"repoName" msgpack:"repoName"`       // The name of the repo
	References  types.PushedReferences `json:"references" msgpack:"references"`   // A list of references pushed
	PusherKeyID string                 `json:"pusherKeyId" msgpack:"pusherKeyId"` // The PGP key of the pusher
	Size        uint64                 `json:"size" msgpack:"size"`               // Total size of all objects pushed
	Timestamp   int64                  `json:"timestamp" msgpack:"timestamp"`     // Unix timestamp
	NodeSig     []byte                 `json:"nodeSig" msgpack:"nodeSig"`         // The signature of the node that created the PushTx
	NodePubKey  string                 `json:"nodePubKey" msgpack:"nodePubKey"`   // The public key of the push tx signer
}

// GetTargetRepo returns the target repository
func (pt *PushTx) GetTargetRepo() types.BareRepo {
	return pt.targetRepo
}

// GetPusherKeyID returns the pusher gpg key ID
func (pt *PushTx) GetPusherKeyID() string {
	return pt.PusherKeyID
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pt *PushTx) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(pt.RepoName, pt.References, pt.PusherKeyID,
		pt.NodeSig, pt.Size, pt.Timestamp)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pt *PushTx) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&pt.RepoName, &pt.References, &pt.PusherKeyID,
		&pt.NodeSig, &pt.Size, &pt.Timestamp)
}

// Bytes returns a serialized version of the object
func (pt *PushTx) Bytes() []byte {
	return util.ObjectToBytes(pt)
}

// LenMinusFee returns the length of the serialized tx minus
// the total length of fee fields.
func (pt *PushTx) LenMinusFee() uint64 {
	var feeFieldsLen = 0
	for _, r := range pt.References {
		feeFieldsLen += len(util.ObjectToBytes(r.Fee))
	}

	return pt.Len() - uint64(feeFieldsLen)
}

// GetRepoName returns the name of the repo receiving the push
func (pt *PushTx) GetRepoName() string {
	return pt.RepoName
}

// GetPushedReferences returns the pushed references
func (pt *PushTx) GetPushedReferences() types.PushedReferences {
	return pt.References
}

// Len returns the length of the serialized tx
func (pt *PushTx) Len() uint64 {
	return uint64(len(pt.Bytes()))
}

// ID returns the hash of the push tx
func (pt *PushTx) ID() util.Hash {
	return util.BytesToHash(util.Blake2b256(pt.Bytes()))
}

// TxSize is the size of the transaction
func (pt *PushTx) TxSize() uint {
	return uint(len(pt.Bytes()))
}

// BillableSize is the size of the transaction + pushed objects
func (pt *PushTx) BillableSize() uint64 {
	return pt.LenMinusFee() + pt.Size
}

// GetSize returns the total pushed objects size
func (pt *PushTx) GetSize() uint64 {
	return pt.Size
}

// TotalFee returns the sum of reference update fees
func (pt *PushTx) TotalFee() util.String {
	sum := decimal.NewFromFloat(0)
	for _, r := range pt.References {
		sum = sum.Add(r.Fee.Decimal())
	}
	return util.String(sum.String())
}
