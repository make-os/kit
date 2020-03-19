package state

import (
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

// BarePushKey returns a PushKey object with zero values
func BarePushKey() *PushKey {
	return &PushKey{
		Scopes:  []string{},
		FeeCap:  "0",
		FeeUsed: "0",
	}
}

// PushKey represents a push key
type PushKey struct {
	util.SerializerHelper `json:"-" msgpack:"-"`
	PubKey                crypto.PublicKey `json:"pubKey" mapstructure:"pubKey" msgpack:"pubKey"`
	Address               util.Address     `json:"address" mapstructure:"address" msgpack:"address"`
	Scopes                []string         `json:"scopes" mapstructure:"scopes" msgpack:"scopes"`
	FeeCap                util.String      `json:"feeCap" mapstructure:"feeCap" msgpack:"feeCap"`
	FeeUsed               util.String      `json:"feeUsed" mapstructure:"feeUsed" msgpack:"feeUsed"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pk *PushKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return pk.EncodeMulti(enc,
		pk.PubKey,
		pk.Address,
		pk.Scopes,
		pk.FeeCap,
		pk.FeeUsed)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pk *PushKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pk.DecodeMulti(dec,
		&pk.PubKey,
		&pk.Address,
		&pk.Scopes,
		&pk.FeeCap,
		&pk.FeeUsed)
}

// Bytes return the serialized equivalent
func (pk *PushKey) Bytes() []byte {
	return util.ToBytes(pk)
}

// IsNil returns true if g fields have zero values
func (pk *PushKey) IsNil() bool {
	return pk.PubKey.IsEmpty() && pk.Address.IsEmpty()
}

// NewGPGPubKeyFromBytes deserialize bz to PushKey
func NewGPGPubKeyFromBytes(bz []byte) (*PushKey, error) {
	var o = &PushKey{}
	return o, util.ToObject(bz, o)
}
