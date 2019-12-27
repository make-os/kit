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
	PushNote  *PushNote `json:"push" mapstructure:"push"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxPush) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.PushNote)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxPush) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.PushNote)
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
func (tx *TxPush) ComputeHash() util.Hash {
	return util.BytesToHash(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxPush) GetHash() util.Hash {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxPush) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in economic calculations
func (tx *TxPush) GetEcoSize() int64 {
	fee := tx.Fee
	tx.Fee = ""
	bz := tx.Bytes()
	tx.Fee = fee
	size := uint64(len(bz))
	pushNoteEcoSize := tx.PushNote.GetEcoSize()
	diff := size - pushNoteEcoSize
	return int64(size - diff)
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxPush) GetSize() int64 {
	return int64(len(tx.Bytes()))
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
