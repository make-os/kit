package core

import (
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// TxRegisterPushKey implements BaseTx, it describes a transaction that registers a push key
type TxRegisterPushKey struct {
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	PublicKey crypto.PublicKey `json:"pubKey" msgpack:"pubKey" mapstructure:"pubKey"`
	Scopes    []string         `json:"scopes" msgpack:"scopes" mapstructure:"scopes"`
	FeeCap    util.String      `json:"feeCap" msgpack:"feeCap" mapstructure:"feeCap"`
}

// NewBareTxRegisterPushKey returns an instance of TxRegisterPushKey with zero values
func NewBareTxRegisterPushKey() *TxRegisterPushKey {
	return &TxRegisterPushKey{
		TxType:   &TxType{Type: TxTypeRegisterPushKey},
		TxCommon: NewBareTxCommon(),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRegisterPushKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.PublicKey,
		tx.Scopes,
		tx.FeeCap)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRegisterPushKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.PublicKey,
		&tx.Scopes,
		&tx.FeeCap)
}

// Bytes returns the serialized transaction
func (tx *TxRegisterPushKey) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRegisterPushKey) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRegisterPushKey) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRegisterPushKey) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRegisterPushKey) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRegisterPushKey) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRegisterPushKey) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRegisterPushKey) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRegisterPushKey) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxRegisterPushKey) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })

	o := objx.New(data)

	// PublicKey: expects string type, base58 encoded
	if pubKeyVal := o.Get("pubKey"); !pubKeyVal.IsNil() {
		if pubKeyVal.IsStr() {
			pubKey, err := crypto.PubKeyFromBase58(pubKeyVal.Str())
			if err != nil {
				return util.FieldError("pubKey", "unable to decode from base58")
			}
			tx.PublicKey = crypto.BytesToPublicKey(pubKey.MustBytes())
		} else {
			return util.FieldError("pubKey", fmt.Sprintf("invalid value type: has %T, "+
				"wants base58 string", pubKeyVal.Inter()))
		}
	}

	// AddScopes: expects string or slice of string types in map
	if scopesVal := o.Get("scopes"); !scopesVal.IsNil() {
		if scopesVal.IsStr() {
			tx.Scopes = strings.Split(scopesVal.Str(), ",")
		} else if scopesVal.IsStrSlice() {
			tx.Scopes = scopesVal.StrSlice()
		} else {
			return util.FieldError("scopes", fmt.Sprintf("invalid value type: has %T, "+
				"wants string|[]string", scopesVal.Inter()))
		}
	}

	// FeeCap: expects int64, float64 or string types in map
	if feeCap := o.Get("feeCap"); !feeCap.IsNil() {
		if feeCap.IsInt64() || feeCap.IsFloat64() {
			tx.FeeCap = util.String(fmt.Sprintf("%v", feeCap.Inter()))
		} else if feeCap.IsStr() {
			tx.FeeCap = util.String(feeCap.Str())
		} else {
			return util.FieldError("feeCap", fmt.Sprintf("invalid value type: has %T, "+
				"wants string|int64|float", feeCap.Inter()))
		}
	}

	return err
}
