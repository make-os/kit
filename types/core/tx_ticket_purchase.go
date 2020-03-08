package core

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// TxTicketPurchase implements BaseTx, it describes a transaction that purchases
// a ticket from the signer or delegates to another address.
type TxTicketPurchase struct {
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxValue  `json:",flatten" msgpack:"-" mapstructure:"-"`
	Delegate  util.PublicKey `json:"delegate" msgpack:"delegate"`
	BLSPubKey []byte         `json:"blsPubKey" msgpack:"blsPubKey"`
}

// NewBareTxTicketPurchase returns an instance of TxTicketPurchase with zero values
func NewBareTxTicketPurchase(ticketType int) *TxTicketPurchase {
	return &TxTicketPurchase{
		TxType:    &TxType{Type: ticketType},
		TxCommon:  NewBareTxCommon(),
		TxValue:   &TxValue{Value: "0"},
		Delegate:  util.EmptyPublicKey,
		BLSPubKey: []byte{},
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
		tx.Delegate.Bytes(),
		tx.BLSPubKey)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxTicketPurchase) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Value,
		&tx.Delegate,
		&tx.BLSPubKey)
}

// Bytes returns the serialized transaction
func (tx *TxTicketPurchase) Bytes() []byte {
	return util.ToBytes(tx)
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
	return tx.GetSize()
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

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxTicketPurchase) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxValue.FromMap(data) })

	o := objx.New(data)

	// Delegate: expects string type in map
	if delVal := o.Get("delegate"); !delVal.IsNil() {
		if delVal.IsStr() {
			pubKey, err := crypto.PubKeyFromBase58(delVal.Str())
			if err != nil {
				return util.FieldError("delegate", "unable to decode from base58")
			}
			tx.Delegate = util.BytesToPublicKey(pubKey.MustBytes())
		} else {
			return util.FieldError("name", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", delVal.Inter()))
		}
	}

	// BLSPubKey: expects string type, hex encoded
	if blsPKVal := o.Get("blsPubKey"); !blsPKVal.IsNil() {
		if blsPKVal.IsStr() {
			tx.BLSPubKey, err = util.FromHex(blsPKVal.Str())
			if err != nil {
				return util.FieldError("blsPubKey", "unable to decode from hex")
			}
		} else {
			return util.FieldError("blsPubKey", fmt.Sprintf("invalid value type: has %T, "+
				"wants hex string", blsPKVal.Inter()))
		}
	}

	return err
}
