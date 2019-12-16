package types

import "github.com/vmihailenco/msgpack"

import "github.com/makeos/mosdef/util"

// BareGPGPubKey returns a GPGPubKey object with zero values
func BareGPGPubKey() *GPGPubKey {
	return &GPGPubKey{}
}

// GPGPubKey represents a GPG public key
type GPGPubKey struct {
	PubKey  string      `json:"pubKey"`
	Address util.String `json:"address"`
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (g *GPGPubKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(g.PubKey, g.Address)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (g *GPGPubKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&g.PubKey, &g.Address)
}

// Bytes return the serialized equivalent
func (g *GPGPubKey) Bytes() []byte {
	return util.ObjectToBytes(g)
}

// IsNil returns true if g fields have zero values
func (g *GPGPubKey) IsNil() bool {
	return g.PubKey == "" && g.Address.Empty()
}

// NewGPGPubKeyFromBytes deserialize bz to GPGPubKey
func NewGPGPubKeyFromBytes(bz []byte) (*GPGPubKey, error) {
	var o = &GPGPubKey{}
	return o, util.BytesToObject(bz, o)
}
