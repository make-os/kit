package repo

import (
	"github.com/makeos/mosdef/util"
	"github.com/shopspring/decimal"
	"github.com/vmihailenco/msgpack"
)

// PushedReferences represents a collection of pushed references
type PushedReferences []*PushedReference

// PushedReference represents a reference that was pushed by git client
type PushedReference struct {
	Name         string      `json:"name" msgpack:"name"`                 // The full name of the reference
	OldObjectID  string      `json:"oldObjId" msgpack:"oldObjId"`         // The object hash of the reference before the push
	NewObjectID  string      `json:"newObjId" msgpack:"newObjId"`         // The object hash of the reference after the push
	Nonce        uint64      `json:"nonce" msgpack:"nonce"`               // The next repo nonce of the reference
	AccountNonce uint64      `json:"accountNonce" msgpack:"accountNonce"` // The pusher's account nonce
	Fee          util.String `json:"fee" msgpack:"fee"`                   // The fee the pusher is willing to pay to validators
	Objects      []string    `json:"objects" msgpack:"objects"`           // A list of objects pushed to the reference
	Sig          string      `json:"sig" msgpack:"sig"`                   // The signature of the pusher
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pr *PushedReference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(pr.Name, pr.OldObjectID, pr.NewObjectID,
		pr.Nonce, pr.AccountNonce, pr.Fee, pr.Objects, pr.Sig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pr *PushedReference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&pr.Name, &pr.OldObjectID, &pr.NewObjectID,
		&pr.Nonce, &pr.AccountNonce, &pr.Fee, &pr.Objects, &pr.Sig)
}

// PushTx represents a repository push request
type PushTx struct {
	RepoName    string           `json:"repoName" msgpack:"repoName"`       // The name of the repo
	References  PushedReferences `json:"references" msgpack:"references"`   // A list of references pushed
	PusherKeyID string           `json:"pusherKeyId" msgpack:"pusherKeyId"` // The PGP key of the pusher
	Size        uint64           `json:"size" msgpack:"size"`               // Total size of all objects pushed
	Timestamp   int64            `json:"timestamp" msgpack:"timestamp"`     // Unix timestamp
	NodeSig     []byte           `json:"nodeSig" msgpack:"nodeSig"`         // The signature of the node that created the PushTx
	NodePubKey  string           `json:"nodePubKey" msgpack:"nodePubKey"`   // The public key of the push tx signer
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

// TotalFee returns the sum of reference update fees
func (pt *PushTx) TotalFee() util.String {
	sum := decimal.NewFromFloat(0)
	for _, r := range pt.References {
		sum = sum.Add(r.Fee.Decimal())
	}
	return util.String(sum.String())
}
