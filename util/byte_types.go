package util

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// Constants
const (
	Length32 = 32
	Length64 = 64
)

// Bytes represents a byte slice
type Bytes []byte

// Bytes returns a slice of bytes
func (b Bytes) Bytes() []byte {
	return b
}

func (u Bytes) MarshalJSON() ([]byte, error) {
	var result string
	if u == nil {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", u)), ",")
	}
	return []byte(result), nil
}

// Big returns the bytes as big integer
func (b Bytes) Big() *big.Int { return new(big.Int).SetBytes(b.Bytes()) }

// Equal checks equality between h and o
func (b Bytes) Equal(o Bytes32) bool { return bytes.Equal(b.Bytes(), o.Bytes()) }

func (b Bytes) String() string { return b.HexStr() }

// HexStr encodes the bytes to hex.
// noPrefix removes '0x' prefix.
func (b Bytes) HexStr(noPrefix ...bool) string { return ToHex(b.Bytes(), noPrefix...) }

// Hex encodes the bytes to hex
func (b Bytes) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(dst, b.Bytes())
	return dst
}

// IsEmpty checks whether the object is empty (having zero values)
func (b Bytes) IsEmpty() bool { return len(b) == 0 }

// Bytes32 represents a 32-bytes value
type Bytes32 [Length32]byte

// EmptyBytes32 is an empty Bytes32
var EmptyBytes32 = Bytes32([Length32]byte{})

// Bytes returns a slice of bytes
func (b Bytes32) Bytes() []byte {
	if b.IsEmpty() {
		return []byte{}
	}
	return b[:]
}

func (b Bytes32) MarshalJSON() ([]byte, error) {
	var result string
	if b.IsEmpty() {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
	}
	return []byte(result), nil
}

// Big returns the bytes as big integer
func (b Bytes32) Big() *big.Int { return new(big.Int).SetBytes(b.Bytes()) }

// Equal checks equality between h and o
func (b Bytes32) Equal(o Bytes32) bool { return bytes.Equal(b.Bytes(), o.Bytes()) }

func (b Bytes32) String() string { return b.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (b Bytes32) HexStr() string {
	return ToHex(b.Bytes())
}

// Hex encodes the bytes to hex
func (b Bytes32) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(dst, b.Bytes())
	return dst
}

// IsEmpty checks whether the object is empty (having zero values)
func (b Bytes32) IsEmpty() bool {
	return b == EmptyBytes32
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
func (b Bytes64) Bytes() []byte {
	if b.IsEmpty() {
		return []byte{}
	}
	return b[:]
}

func (b Bytes64) MarshalJSON() ([]byte, error) {
	var result string
	if b.IsEmpty() {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
	}
	return []byte(result), nil
}

// Big returns the bytes as big integer
func (b Bytes64) Big() *big.Int { return new(big.Int).SetBytes(b.Bytes()) }

// Equal checks equality between h and o
func (b Bytes64) Equal(o Bytes64) bool { return bytes.Equal(b.Bytes(), o.Bytes()) }

func (b Bytes64) String() string { return b.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (b Bytes64) HexStr() string {
	return ToHex(b.Bytes())
}

// Hex encodes the bytes to hex
func (b Bytes64) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(dst, b.Bytes())
	return dst
}

// IsEmpty checks whether the object is empty (having zero values)
func (b Bytes64) IsEmpty() bool {
	return b == EmptyBytes64
}

// BytesToBytes64 copies b to a Bytes64
func BytesToBytes64(b []byte) Bytes64 {
	var h Bytes64
	copy(h[:], b)
	return h
}

// BlockNonce represents a 64-bit
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

func (n BlockNonce) MarshalJSON() ([]byte, error) {
	var result string
	if n == [8]byte{} {
		result = "null"
	} else {
		result = strings.Join(strings.Fields(fmt.Sprintf("%d", n)), ",")
	}
	return []byte(result), nil
}
