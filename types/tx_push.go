package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxPush implements BaseTx, it describes a transaction that creates a
// repository for the signer
type TxPush struct {
	*TxCommon `json:"-" mapstructure:"-"`
	*TxType   `json:"-" msgpack:"-"`
	PushNote  *PushNote `json:"pushNote" mapstructure:"pushNote"`
	PushOKs   []*PushOK `json:"endorsements" mapstructure:"endorsements"`
}

// NewBareTxPush returns an instance of TxPush with zero values
func NewBareTxPush() *TxPush {
	return &TxPush{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypePush},
		PushNote: &PushNote{},
		PushOKs:  []*PushOK{},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxPush) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.SenderPubKey,
		tx.Timestamp,
		tx.PushNote,
		tx.PushOKs,
		tx.Sig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxPush) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.SenderPubKey,
		&tx.Timestamp,
		&tx.PushNote,
		&tx.PushOKs,
		&tx.Sig)
}

// Bytes returns the serialized transaction
func (tx *TxPush) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxPush) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxPush) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxPush) GetHash() util.Bytes32 {
	return tx.PushNote.ID()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxPush) GetID() string {
	return tx.PushNote.ID().String()
}

// GetEcoSize returns the size of the transaction for use in economic calculations
func (tx *TxPush) GetEcoSize() int64 {
	fee := tx.Fee
	tx.Fee = ""

	bz := tx.Bytes()
	size := uint64(len(bz))
	pushNoteEcoSize := tx.PushNote.GetEcoSize()
	diff := size - pushNoteEcoSize

	tx.Fee = fee
	return int64(size - diff)
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxPush) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// GetTimestamp return the transaction creation unix timestamp.
// Because TxPush is a wrapper transaction, we use the push note timestamp
func (tx *TxPush) GetTimestamp() int64 {
	return tx.PushNote.Timestamp
}

// GetNonce returns the transaction nonce.
// Because TxPush is a wrapper transaction, we use the Account nonce of the pusher
// which is found in anyone of the pushed reference
func (tx *TxPush) GetNonce() uint64 {
	return tx.PushNote.References[0].AccountNonce
}

// GetFrom returns the address of the transaction sender
// Because TxPush is a wrapper transaction, we use the pusher's public key ID
// Panics if sender's public key is invalid.
func (tx *TxPush) GetFrom() util.String {
	return util.String(tx.PushNote.PusherKeyID)
}

// Sign signs the transaction
func (tx *TxPush) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxPush) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
