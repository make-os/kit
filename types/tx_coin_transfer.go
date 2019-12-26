package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxCoinTransfer implements BaseTx, it describes a transaction that transfers
// the native coin from one account to another.
type TxCoinTransfer struct {
	*TxType      `json:"-" msgpack:"-"`
	*TxCommon    `json:"-" mapstructure:"-"`
	*TxRecipient `json:"-" mapstructure:"-"`
	*TxValue     `json:"-" mapstructure:"-"`
}

// NewBareTxCoinTransfer returns an instance of TxCoinTransfer with zero values
func NewBareTxCoinTransfer() *TxCoinTransfer {
	return &TxCoinTransfer{
		TxType:      &TxType{Type: TxTypeCoinTransfer},
		TxCommon:    NewBareTxCommon(),
		TxRecipient: &TxRecipient{To: ""},
		TxValue:     &TxValue{Value: "0"},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxCoinTransfer) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.To,
		tx.Value)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxCoinTransfer) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.To,
		&tx.Value)
}

// Bytes returns the serialized transaction
func (tx *TxCoinTransfer) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxCoinTransfer) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxCoinTransfer) ComputeHash() util.Hash {
	return util.BytesToHash(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxCoinTransfer) GetHash() util.Hash {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxCoinTransfer) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxCoinTransfer) GetEcoSize() int64 {
	fee := tx.Fee
	tx.Fee = ""
	bz := tx.Bytes()
	tx.Fee = fee
	return int64(len(bz))
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxCoinTransfer) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxCoinTransfer) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxCoinTransfer) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
