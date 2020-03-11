package util

import (
	"bytes"
	"encoding/hex"
	"math/big"
)

// Constants
const (
	Length32 = 32
	Length64 = 64
)

// Bytes32 represents a 32-bytes value
type Bytes32 [Length32]byte

// EmptyBytes32 is an empty Bytes32
var EmptyBytes32 = Bytes32([Length32]byte{})

// Bytes returns a slice of bytes
func (h Bytes32) Bytes() []byte {
	if h.IsEmpty() {
		return []byte{}
	}
	return h[:]
}

// Big returns the bytes as big integer
func (h Bytes32) Big() *big.Int { return new(big.Int).SetBytes(h.Bytes()) }

// Equal checks equality between h and o
func (h Bytes32) Equal(o Bytes32) bool { return bytes.Equal(h.Bytes(), o.Bytes()) }

func (h Bytes32) String() string { return h.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (h Bytes32) HexStr() string {
	return ToHex(h.Bytes())
}

// Hex encodes the bytes to hex
func (h Bytes32) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(dst, h.Bytes())
	return dst
}

// IsEmpty checks whether the object is empty (having zero values)
func (h Bytes32) IsEmpty() bool {
	return h == EmptyBytes32
}

// HexToBytes32 creates an hex value to Bytes32
func HexToBytes32(hex string) (Bytes32, error) {
	bs, err := FromHex(hex)
	if err != nil {
		return EmptyBytes32, err
	}
	return BytesToBytes32(bs), nil
}

// BytesToBytes32 copies b to a Bytes32
func BytesToBytes32(b []byte) Bytes32 {
	var h Bytes32
	copy(h[:], b)
	return h
}

// StrToBytes32 converts a string to a Bytes32
func StrToBytes32(s string) Bytes32 {
	return BytesToBytes32([]byte(s))
}

// Bytes64 represents a 32-bytes value
type Bytes64 [Length64]byte

// EmptyBytes64 is an empty Bytes64
var EmptyBytes64 = Bytes64([Length64]byte{})

// Bytes returns a slice of bytes
func (h Bytes64) Bytes() []byte {
	if h.IsEmpty() {
		return []byte{}
	}
	return h[:]
}

// Big returns the bytes as big integer
func (h Bytes64) Big() *big.Int { return new(big.Int).SetBytes(h.Bytes()) }

// Equal checks equality between h and o
func (h Bytes64) Equal(o Bytes64) bool { return bytes.Equal(h.Bytes(), o.Bytes()) }

func (h Bytes64) String() string { return h.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (h Bytes64) HexStr() string {
	return ToHex(h.Bytes())
}

// Hex encodes the bytes to hex
func (h Bytes64) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(dst, h.Bytes())
	return dst
}

// IsEmpty checks whether the object is empty (having zero values)
func (h Bytes64) IsEmpty() bool {
	return h == EmptyBytes64
}

// BytesToBytes64 copies b to a Bytes64
func BytesToBytes64(b []byte) Bytes64 {
	var h Bytes64
	copy(h[:], b)
	return h
}
