package ed25519

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/make-os/kit/pkgs/bech32"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
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

func (pk PublicKey) MarshalJSON() ([]byte, error) {
	var result string
	if pk.IsEmpty() {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", pk)), ",")
	}
	return []byte(result), nil
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
func (pk PublicKey) MustAddress() identifier.Address {
	return MustPubKeyFromBytes(pk.Bytes()).Addr()
}

// MustPushKeyAddress derives a push key address from the key.
// Panics on failure.
func (pk PublicKey) MustPushKeyAddress() identifier.Address {
	return MustPubKeyFromBytes(pk.Bytes()).PushAddr()
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

// CreatePushKeyID returns bech32 address corresponding to a push key.
// Panics if pk is not a valid ed25519 public key
func CreatePushKeyID(pk PublicKey) string {
	return MustPubKeyFromBytes(pk.Bytes()).PushAddr().String()
}

// CreatePushKeyID returns bech32 address corresponding to a push key.
// Panics if pk is not a valid ed25519 public key
func BytesToPushKeyID(pk []byte) string {
	encoded, err := bech32.ConvertAndEncode(constants.PushAddrHRP, pk)
	if err != nil {
		panic(err)
	}
	return encoded
}

// PushKey represents push key
type PushKey []byte

// String returns the push key ID as a string
func (p PushKey) String() string {
	return BytesToPushKeyID(p)
}

func (p PushKey) MarshalJSON() ([]byte, error) {
	var result string
	if p == nil {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", p)), ",")
	}
	return []byte(result), nil
}
