package state

import (
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// BareGPGPubKey returns a GPGPubKey object with zero values
func BareGPGPubKey() *GPGPubKey {
	return &GPGPubKey{
		Scopes:  []string{},
		FeeCap:  "0",
		FeeUsed: "0",
	}
}

// GPGPubKey represents a GPG public key
type GPGPubKey struct {
	util.SerializerHelper `json:"-" msgpack:"-"`
	PubKey                string      `json:"pubKey" mapstructure:"pubKey" msgpack:"pubKey"`
	Address               util.String `json:"address" mapstructure:"address" msgpack:"address"`
	Scopes                []string    `json:"scopes" mapstructure:"scopes" msgpack:"scopes"`
	FeeCap                util.String `json:"feeCap" mapstructure:"feeCap" msgpack:"feeCap"`
	FeeUsed               util.String `json:"feeUsed" mapstructure:"feeUsed" msgpack:"feeUsed"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (g *GPGPubKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return g.EncodeMulti(enc,
		g.PubKey,
		g.Address,
		g.Scopes,
		g.FeeCap,
		g.FeeUsed)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (g *GPGPubKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return g.DecodeMulti(dec,
		&g.PubKey,
		&g.Address,
		&g.Scopes,
		&g.FeeCap,
		&g.FeeUsed)
}

// Bytes return the serialized equivalent
func (g *GPGPubKey) Bytes() []byte {
	return util.ToBytes(g)
}

// IsNil returns true if g fields have zero values
func (g *GPGPubKey) IsNil() bool {
	return g.PubKey == "" && g.Address.Empty()
}

// NewGPGPubKeyFromBytes deserialize bz to GPGPubKey
func NewGPGPubKeyFromBytes(bz []byte) (*GPGPubKey, error) {
	var o = &GPGPubKey{}
	return o, util.ToObject(bz, o)
}
