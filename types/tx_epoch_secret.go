package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxEpochSecret implements BaseTx, it describes a transaction that contains
// random secrets used for validator selection.
type TxEpochSecret struct {
	*TxType        `json:"-" msgpack:"-" mapstructure:"-"`
	*TxCommon      `json:"-" msgpack:"-" mapstructure:"-"`
	Secret         []byte `json:"secret,omitempty" msgpack:"secret,omitempty"`
	PreviousSecret []byte `json:"previousSecret,omitempty" msgpack:"previousSecret,omitempty"`
	SecretRound    uint64 `json:"secretRound,omitempty" msgpack:"secretRound,omitempty"`
}

// NewBareTxEpochSecret returns an instance of TxEpochSecret with zero values
func NewBareTxEpochSecret() *TxEpochSecret {
	return &TxEpochSecret{
		TxCommon:       NewBareTxCommon(),
		TxType:         &TxType{Type: TxTypeEpochSecret},
		Secret:         []byte{},
		PreviousSecret: []byte{},
		SecretRound:    0,
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxEpochSecret) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Secret,
		tx.PreviousSecret,
		tx.SecretRound,
		tx.SenderPubKey,
		tx.Sig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxEpochSecret) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Secret,
		&tx.PreviousSecret,
		&tx.SecretRound,
		&tx.SenderPubKey,
		&tx.Sig)
}

// Bytes returns the serialized transaction
func (tx *TxEpochSecret) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxEpochSecret) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxEpochSecret) ComputeHash() util.Bytes32 {
	return util.BytesToHash(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxEpochSecret) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxEpochSecret) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxEpochSecret) GetEcoSize() int64 {
	panic("not implemented")
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxEpochSecret) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxEpochSecret) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxEpochSecret) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
