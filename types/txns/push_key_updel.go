package txns

import (
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

// TxUpDelPushKey implements BaseTx, it describes a transaction used to update
// or delete a registered push key
type TxUpDelPushKey struct {
	*TxCommon    `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType      `json:",flatten" msgpack:"-" mapstructure:"-"`
	ID           string      `json:"id" msgpack:"id" mapstructure:"id"`
	AddScopes    []string    `json:"addScopes" msgpack:"addScopes" mapstructure:"addScopes"`
	RemoveScopes []int       `json:"removeScopes" msgpack:"removeScopes" mapstructure:"removeScopes"`
	FeeCap       util.String `json:"feeCap" msgpack:"feeCap" mapstructure:"feeCap"`
	Delete       bool        `json:"delete" msgpack:"delete" mapstructure:"delete"`
}

// NewBareTxUpDelPushKey returns an instance of TxUpDelPushKey with zero values
func NewBareTxUpDelPushKey() *TxUpDelPushKey {
	return &TxUpDelPushKey{
		TxType:   &TxType{Type: TxTypeUpDelPushKey},
		TxCommon: NewBareTxCommon(),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxUpDelPushKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.ID,
		tx.AddScopes,
		tx.RemoveScopes,
		tx.FeeCap,
		tx.Delete)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxUpDelPushKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.ID,
		&tx.AddScopes,
		&tx.RemoveScopes,
		&tx.FeeCap,
		&tx.Delete)
}

// Bytes returns the serialized transaction
func (tx *TxUpDelPushKey) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxUpDelPushKey) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxUpDelPushKey) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxUpDelPushKey) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxUpDelPushKey) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxUpDelPushKey) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxUpDelPushKey) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxUpDelPushKey) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxUpDelPushKey) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxUpDelPushKey) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = errors.CallIfNil(err, func() error { return tx.TxType.FromMap(data) })
	err = errors.CallIfNil(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
