package core

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// TxAddGPGPubKey implements BaseTx, it describes a transaction that registers a
// gpg key to the transaction signer
type TxAddGPGPubKey struct {
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	PublicKey string `json:"pubKey" msgpack:"pubKey" mapstructure:"pubKey"`
}

// NewBareTxAddGPGPubKey returns an instance of TxAddGPGPubKey with zero values
func NewBareTxAddGPGPubKey() *TxAddGPGPubKey {
	return &TxAddGPGPubKey{
		TxType:    &TxType{Type: TxTypeAddGPGPubKey},
		TxCommon:  NewBareTxCommon(),
		PublicKey: "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxAddGPGPubKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.PublicKey)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxAddGPGPubKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.PublicKey)
}

// Bytes returns the serialized transaction
func (tx *TxAddGPGPubKey) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxAddGPGPubKey) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxAddGPGPubKey) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxAddGPGPubKey) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxAddGPGPubKey) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxAddGPGPubKey) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxAddGPGPubKey) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxAddGPGPubKey) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxAddGPGPubKey) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxAddGPGPubKey) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })

	o := objx.New(data)

	// PublicKey: expects string type in map
	if pubKeyVal := o.Get("pubKey"); !pubKeyVal.IsNil() {
		if pubKeyVal.IsStr() {
			tx.PublicKey = pubKeyVal.Str()
		} else {
			return util.FieldError("pubKey", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", pubKeyVal.Inter()))
		}
	}

	return err
}
