package msgs

import (
	"gitlab.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

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

