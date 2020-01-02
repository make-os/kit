package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxTicketPurchase implements BaseTx, it describes a transaction that purchases
// a ticket from the signer or delegates to another address.
type TxTicketPurchase struct {
	*TxType   `json:"-" msgpack:"-" mapstructure:"-"`
	*TxCommon `json:"-" msgpack:"-" mapstructure:"-"`
	*TxValue  `json:"-" msgpack:"-" mapstructure:"-"`
	Delegate  string `json:"delegate" msgpack:"delegate"`
}

// NewBareTxTicketPurchase returns an instance of TxTicketPurchase with zero values
func NewBareTxTicketPurchase(ticketType int) *TxTicketPurchase {
	return &TxTicketPurchase{
		TxType:   &TxType{Type: ticketType},
		TxCommon: NewBareTxCommon(),
		TxValue:  &TxValue{Value: "0"},
		Delegate: "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxTicketPurchase) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Value,
		tx.Delegate)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxTicketPurchase) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Value,
		&tx.Delegate)
}

// Bytes returns the serialized transaction
func (tx *TxTicketPurchase) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxTicketPurchase) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxTicketPurchase) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxTicketPurchase) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxTicketPurchase) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxTicketPurchase) GetEcoSize() int64 {
	fee := tx.Fee
	tx.Fee = ""
	bz := tx.Bytes()
	tx.Fee = fee
	return int64(len(bz))
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxTicketPurchase) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxTicketPurchase) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxTicketPurchase) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
