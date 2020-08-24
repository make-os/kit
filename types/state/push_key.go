package state

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/vmihailenco/msgpack"
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
	util.CodecUtil `json:"-" msgpack:"-"`
	PubKey         crypto.PublicKey   `json:"pubKey,omitempty" mapstructure:"pubKey,omitempty" msgpack:"pubKey,omitempty"`
	Address        identifier.Address `json:"address,omitempty" mapstructure:"address,omitempty" msgpack:"address,omitempty"`
	Scopes         []string           `json:"scopes,omitempty" mapstructure:"scopes,omitempty" msgpack:"scopes,omitempty"`
	FeeCap         util.String        `json:"feeCap,omitempty" mapstructure:"feeCap,omitempty" msgpack:"feeCap,omitempty"`
	FeeUsed        util.String        `json:"feeUsed,omitempty" mapstructure:"feeUsed,omitempty" msgpack:"feeUsed,omitempty"`
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

// NewPushKeyFromBytes deserialize bz to PushKey
func NewPushKeyFromBytes(bz []byte) (*PushKey, error) {
	var o = &PushKey{}
	return o, util.ToObject(bz, o)
}
