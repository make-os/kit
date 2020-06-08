package txns

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
	"gitlab.com/makeos/mosdef/util/crypto"
)

// TxRepoCreate implements BaseTx, it describes a transaction that creates a
// repository for the signer
type TxRepoCreate struct {
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxValue  `json:",flatten" msgpack:"-" mapstructure:"-"`
	Name      string                 `json:"name" msgpack:"name" mapstructure:"name"`
	Config    map[string]interface{} `json:"config" msgpack:"config" mapstructure:"config"`
}

// NewBareTxRepoCreate returns an instance of TxRepoCreate with zero values
func NewBareTxRepoCreate() *TxRepoCreate {
	return &TxRepoCreate{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypeRepoCreate},
		TxValue:  &TxValue{Value: "0"},
		Name:     "",
		Config:   make(map[string]interface{}),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoCreate) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Value,
		tx.Name,
		tx.Config)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoCreate) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Value,
		&tx.Name,
		&tx.Config)
}

// Bytes returns the serialized transaction
func (tx *TxRepoCreate) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoCreate) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoCreate) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(crypto.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoCreate) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoCreate) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoCreate) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoCreate) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoCreate) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoCreate) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxRepoCreate) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxValue.FromMap(data) })

	o := objx.New(data)

	// Name: expects string type in map
	if nameVal := o.Get("name"); !nameVal.IsNil() {
		if nameVal.IsStr() {
			tx.Name = nameVal.Str()
		} else {
			return util.FieldError("name", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", nameVal.Inter()))
		}
	}

	// Config: expects map type in map_
	if configVal := o.Get("config"); !configVal.IsNil() {
		if configVal.IsObjxMap() {
			tx.Config = configVal.ObjxMap()
		} else {
			return util.FieldError("config", fmt.Sprintf("invalid value type: has %T, "+
				"wants map", configVal.Inter()))
		}
	}

	return err
}
