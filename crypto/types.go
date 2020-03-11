package crypto

import (
	"bytes"
	"encoding/hex"

	"gitlab.com/makeos/mosdef/util"
)

// PublicKey represents a 32-byte ED25519 public key
type PublicKey [util.Length32]byte

// EmptyPublicKey is an empty PublicKey
var EmptyPublicKey = PublicKey([util.Length32]byte{})

// Bytes returns a slice of bytes
func (pk PublicKey) Bytes() []byte {
	if pk.IsEmpty() {
		return []byte{}
	}
	return pk[:]
}

// ToBytes32 convert PublicKey to Bytes32
func (pk PublicKey) ToBytes32() util.Bytes32 {
	return util.BytesToBytes32(pk.Bytes())
}

// Equal checks equality between h and o
func (pk PublicKey) Equal(o PublicKey) bool { return bytes.Equal(pk.Bytes(), o.Bytes()) }

func (pk PublicKey) String() string { return pk.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (pk PublicKey) HexStr() string {
	return util.ToHex(pk.Bytes())
}

// Hex encodes the bytes to hex
func (pk PublicKey) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(pk)))
	hex.Encode(dst, pk.Bytes())
	return dst
}

// MustAddress derives an address from the key.
// Panics on failure.
func (pk PublicKey) MustAddress() util.Address {
	return MustPubKeyFromBytes(pk.Bytes()).Addr()
}

// IsEmpty checks whether the object is empty (having zero values)
func (pk PublicKey) IsEmpty() bool {
	return pk == EmptyPublicKey
}

// BytesToPublicKey copies b to a PublicKey
func BytesToPublicKey(b []byte) PublicKey {
	var h PublicKey
	copy(h[:], b)
	return h
}

// StrToPublicKey converts a string to a PublicKey
func StrToPublicKey(s string) PublicKey {
	return BytesToPublicKey([]byte(s))
}
