package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/ripemd160"

	"golang.org/x/crypto/blake2b"

	"github.com/tendermint/tendermint/libs/bech32"
)

// Constants
const (
	Length32   = 32
	Length64   = 64
	GPGAddrHRP = "gpg"
)

// PublicKey represents a 32-byte ED25519 public key
type PublicKey [Length32]byte

// EmptyPublicKey is an empty PublicKey
var EmptyPublicKey = PublicKey([Length32]byte{})

// Bytes returns a slice of bytes
func (pk PublicKey) Bytes() []byte {
	if pk.IsEmpty() {
		return []byte{}
	}
	return pk[:]
}

// ToBytes32 convert PublicKey to Bytes32
func (pk PublicKey) ToBytes32() Bytes32 {
	return BytesToBytes32(pk.Bytes())
}

// Equal checks equality between h and o
func (pk PublicKey) Equal(o PublicKey) bool { return bytes.Equal(pk.Bytes(), o.Bytes()) }

func (pk PublicKey) String() string { return pk.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (pk PublicKey) HexStr() string {
	return ToHex(pk.Bytes())
}

// Hex encodes the bytes to hex
func (pk PublicKey) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(pk)))
	hex.Encode(dst, pk.Bytes())
	return dst
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

// SS returns a short version of HexStr with the middle
// characters truncated when length is at least 32
func (h Bytes32) SS() string {
	s := h.HexStr()
	if len(s) >= 32 {
		return fmt.Sprintf("%s...%s", string(s)[0:10], string(s)[len(s)-10:])
	}
	return s
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

// Encrypt encrypts a plaintext
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {

	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	cipherText := make([]byte, aes.BlockSize+len(plaintext))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(c, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plaintext)
	return cipherText, nil
}

// Unlock decrypts a ciphertext
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {

	iv := ciphertext[:aes.BlockSize]
	data := ciphertext[aes.BlockSize:]

	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(c, iv)
	stream.XORKeyStream(data, data)
	return data, nil
}

// Blake2b256 returns blake2b 256 bytes hash of v
func Blake2b256(v []byte) []byte {
	hash, _ := blake2b.New256(nil)
	if _, err := hash.Write(v); err != nil {
		panic(err)
	}
	return hash.Sum(nil)
}

// Hash20 returns 20 bytes hash derived from truncating sha512 output of v
func Hash20(v []byte) []byte {
	h := sha512.New()
	h.Write(v)
	return h.Sum(nil)[:20]
}

// Hash20Hex is like Hash20 but returns hex output
func Hash20Hex(v []byte) string {
	return fmt.Sprintf("%x", Hash20(v))
}

// RIPEMD160 returns RIPEMD160 (20 bytes) hash of v
func RIPEMD160(v []byte) []byte {
	h := ripemd160.New()
	h.Write(v)
	return h.Sum(nil)
}

// RSAPubKeyID returns bech32 encoding of the key with HRP=gpg
func RSAPubKeyID(pk *rsa.PublicKey) string {
	hash20 := RSAPubKeyIDRaw(pk)
	id, err := bech32.ConvertAndEncode(GPGAddrHRP, hash20)
	if err != nil {
		panic(err)
	}
	return id
}

// MustDecodeRSAPubKeyID decodes an RSA public key id to bytes; panics if
// decoding fails.
func MustDecodeRSAPubKeyID(id string) []byte {
	_, bz, err := bech32.DecodeAndConvert(id)
	if err != nil {
		panic(err)
	}
	return bz
}

// IsValidRSAPubKey checks whether the given id is a valid bech32 encoded string
// used for representing an RSA public key
func IsValidRSAPubKey(id string) bool {
	hrp, bz, err := bech32.DecodeAndConvert(id)
	if err != nil {
		return false
	}
	if hrp != GPGAddrHRP {
		return false
	}
	if len(bz) != 20 {
		return false
	}
	return true
}

// MustToRSAPubKeyID takes a 20 bytes value and returns a bech32 address; panics
// if decoding fails.
func MustToRSAPubKeyID(hash20 []byte) string {
	id, err := bech32.ConvertAndEncode(GPGAddrHRP, hash20)
	if err != nil {
		panic(err)
	}
	return id
}

// RSAPubKeyIDRaw returns a 20 bytes fingerprint of the public key
func RSAPubKeyIDRaw(pk *rsa.PublicKey) []byte {
	return RIPEMD160(pk.N.Bytes())
}
