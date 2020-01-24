package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxNamespaceDomainUpdate implements BaseTx, it describes a transaction for acquiring a namespace
type TxNamespaceDomainUpdate struct {
	*TxType   `json:"-" msgpack:"-" mapstructure:"-"`
	*TxCommon `json:"-" msgpack:"-" mapstructure:"-"`
	Name      string            `json:"name" msgpack:"name"`
	Domains   map[string]string `json:"domains" msgpack:"domains"`
}

// NewBareTxNamespaceDomainUpdate returns an instance of TxNamespaceDomainUpdate with zero values
func NewBareTxNamespaceDomainUpdate() *TxNamespaceDomainUpdate {
	return &TxNamespaceDomainUpdate{
		TxType:   &TxType{Type: TxTypeNSDomainUpdate},
		TxCommon: NewBareTxCommon(),
		Name:     "",
		Domains:  make(map[string]string),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxNamespaceDomainUpdate) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Name,
		tx.Domains)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxNamespaceDomainUpdate) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Name,
		&tx.Domains)
}

// Bytes returns the serialized transaction
func (tx *TxNamespaceDomainUpdate) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxNamespaceDomainUpdate) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxNamespaceDomainUpdate) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxNamespaceDomainUpdate) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxNamespaceDomainUpdate) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxNamespaceDomainUpdate) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxNamespaceDomainUpdate) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxNamespaceDomainUpdate) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxNamespaceDomainUpdate) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
