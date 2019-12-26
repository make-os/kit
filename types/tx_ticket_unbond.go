package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxTicketUnbond implements BaseTx, it describes a transaction that unbonds a
// staked coin owned by the signer
type TxTicketUnbond struct {
	*TxType    `json:"-" msgpack:"-"`
	*TxCommon  `json:"-" mapstructure:"-"`
	TicketHash string `json:"hash" msgpack:"hash"`
}

// NewBareTxTicketUnbond returns an instance of TxTicketUnbond with zero values
func NewBareTxTicketUnbond(ticketType int) *TxTicketUnbond {
	return &TxTicketUnbond{
		TxType:     &TxType{Type: ticketType},
		TxCommon:   NewBareTxCommon(),
		TicketHash: "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxTicketUnbond) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.TicketHash)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxTicketUnbond) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.TicketHash)
}

// Bytes returns the serialized transaction
func (tx *TxTicketUnbond) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxTicketUnbond) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxTicketUnbond) ComputeHash() util.Hash {
	return util.BytesToHash(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxTicketUnbond) GetHash() util.Hash {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxTicketUnbond) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxTicketUnbond) GetEcoSize() int64 {
	fee := tx.Fee
	tx.Fee = ""
	bz := tx.Bytes()
	tx.Fee = fee
	return int64(len(bz))
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxTicketUnbond) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxTicketUnbond) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxTicketUnbond) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
