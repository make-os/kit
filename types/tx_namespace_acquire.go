package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxNamespaceAcquire implements BaseTx, it describes a transaction for acquiring a namespace
type TxNamespaceAcquire struct {
	*TxType           `json:"-" msgpack:"-" mapstructure:"-"`
	*TxCommon         `json:"-" msgpack:"-" mapstructure:"-"`
	*TxValue          `json:"-" msgpack:"-" mapstructure:"-"`
	Name              string            `json:"name" msgpack:"name"`
	TransferToRepo    string            `json:"transferToRepo" msgpack:"transferToRepo"`
	TransferToAccount string            `json:"transferToAccount" msgpack:"transferToAccount"`
	Domains           map[string]string `json:"domains" msgpack:"domains"`
}

// NewBareTxNamespaceAcquire returns an instance of TxNamespaceAcquire with zero values
func NewBareTxNamespaceAcquire() *TxNamespaceAcquire {
	return &TxNamespaceAcquire{
		TxType:            &TxType{Type: TxTypeNSAcquire},
		TxCommon:          NewBareTxCommon(),
		TxValue:           &TxValue{Value: "0"},
		Name:              "",
		TransferToRepo:    "",
		TransferToAccount: "",
		Domains:           make(map[string]string),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxNamespaceAcquire) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Name,
		tx.Value,
		tx.TransferToRepo,
		tx.TransferToAccount,
		tx.Domains)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxNamespaceAcquire) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Name,
		&tx.Value,
		&tx.TransferToRepo,
		&tx.TransferToAccount,
		&tx.Domains)
}

// Bytes returns the serialized transaction
func (tx *TxNamespaceAcquire) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxNamespaceAcquire) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxNamespaceAcquire) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxNamespaceAcquire) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxNamespaceAcquire) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxNamespaceAcquire) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxNamespaceAcquire) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxNamespaceAcquire) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxNamespaceAcquire) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
