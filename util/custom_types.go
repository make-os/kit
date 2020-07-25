package util

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/lobe/util/identifier"
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

// HexBytes wraps b in HexBytes
func (b Bytes32) ToHexBytes() HexBytes {
	return b.Bytes()
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

// HexBytes represents a slice of bytes
type HexBytes []byte

// String returns the hex encoded version of h
func (h HexBytes) String() string {
	return ToHex(h)
}

// Bytes returns bytes
func (h HexBytes) Bytes() []byte {
	return h
}

func (h HexBytes) MarshalJSON() ([]byte, error) {
	var result string
	if len(h) == 0 {
		result = "null"
	} else {
		result = `"` + h.String() + `"`
	}
	return []byte(result), nil
}

// Equal checks equality between h and o
func (h HexBytes) Equal(o HexBytes) bool { return bytes.Equal(h, o) }

// IsEmpty checks whether h is empty
func (h HexBytes) IsEmpty() bool { return len(h) == 0 }

// StrToHexBytes converts a string to a HexBytes
func StrToHexBytes(s string) HexBytes {
	return []byte(s)
}

// UInt64 wraps uint64
type UInt64 uint64

func (i *UInt64) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	var v uint64
	v, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}
	i.Set(v)

	return nil
}

// MarshalJSON marshals for JSON
func (i UInt64) MarshalJSON() ([]byte, error) {
	var result string
	if i == 0 {
		result = `"0"`
	} else {
		result = `"` + fmt.Sprintf("%d", i) + `"`
	}
	return []byte(result), nil
}

// UInt64 casts and returns uint64
func (i *UInt64) UInt64() uint64 {
	return uint64(*i)
}

// IsZero checks if the value is zero
func (i *UInt64) IsZero() bool {
	return uint64(*i) == uint64(0)
}

// Set sets the value
func (i *UInt64) Set(v uint64) {
	*i = UInt64(v)
}

// Int64 wraps int64
type Int64 int64

func (i *Int64) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	var v int64
	v, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}
	i.Set(v)

	return nil
}

// MarshalJSON marshals for JSON
func (i Int64) MarshalJSON() ([]byte, error) {
	var result string
	if i == 0 {
		result = `"0"`
	} else {
		result = `"` + fmt.Sprintf("%d", i) + `"`
	}
	return []byte(result), nil
}

// Int64 casts and returns int64
func (i *Int64) Int64() int64 {
	return int64(*i)
}

// Set sets the value
func (i *Int64) Set(v int64) {
	*i = Int64(v)
}

// String represents a custom string
type String string

// Bytes returns the bytes equivalent of the string
func (s String) Bytes() []byte {
	return []byte(s)
}

// Address converts the String to an Address
func (s String) Address() identifier.Address {
	return identifier.Address(s)
}

// Equal check whether s and o are the same
func (s String) Equal(o String) bool {
	return s.String() == o.String()
}

func (s String) String() string {
	return string(s)
}

// IsZero returns true if str is empty or equal "0"
func (s String) IsZero() bool {
	return IsZeroString(string(s))
}

// IsNumeric checks whether s is numeric
func (s String) IsNumeric() bool {
	return govalidator.IsFloat(s.String())
}

// Empty returns true if the string is empty
func (s String) Empty() bool {
	return len(s) == 0
}

// SS returns a short version of String() with the middle
// characters truncated when length is at least 32
func (s String) SS() string {
	if len(s) >= 32 {
		return fmt.Sprintf("%s...%s", string(s)[0:10], string(s)[len(s)-10:])
	}
	return string(s)
}

// Decimal returns the decimal representation of the string.
// Panics if string failed to be converted to decimal.
func (s String) Decimal() decimal.Decimal {
	return StrToDec(s.String())
}

// Float returns the float equivalent of the numeric value.
// Panics if not convertible to float64
func (s String) Float() float64 {
	f, err := strconv.ParseFloat(string(s), 64)
	if err != nil {
		panic(err)
	}
	return f
}

// IsDecimal checks whether the string can be converted to Decimal
func (s String) IsDecimal() bool {
	return govalidator.IsFloat(string(s))
}
