package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxEpochSeed implements BaseTx, it describes a transaction that contains
// random seed used for validator selection.
type TxEpochSeed struct {
	*TxType   `json:"-" msgpack:"-" mapstructure:"-"`
	*TxCommon `json:"-" msgpack:"-" mapstructure:"-"`
	Output    util.Bytes32 `json:"output,omitempty" msgpack:"output,omitempty"`
	Proof     []byte       `json:"proof,omitempty" msgpack:"proof,omitempty"`
}

// NewBareTxEpochSeed returns an instance of TxEpochSeed with zero values
func NewBareTxEpochSeed() *TxEpochSeed {
	return &TxEpochSeed{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypeEpochSeed},
		Proof:    []byte{},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxEpochSeed) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Output,
		tx.Proof)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxEpochSeed) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Output,
		&tx.Proof)
}

// Bytes returns the serialized transaction
func (tx *TxEpochSeed) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxEpochSeed) GetBytesNoSig() []byte {
	panic("not implemented")
}

// ComputeHash computes the hash of the transaction
func (tx *TxEpochSeed) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxEpochSeed) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxEpochSeed) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxEpochSeed) GetEcoSize() int64 {
	panic("not implemented")
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxEpochSeed) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxEpochSeed) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxEpochSeed) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
