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

	crypto "github.com/libp2p/go-libp2p-crypto"
)

// Fixed bytes array lengths
const (
	Length32 = 32
	Length64 = 64
)

// Bytes32 represents a 32-bytes value
type Bytes32 [Length32]byte

// EmptyBytes32 is an empty Bytes32
var EmptyBytes32 = Bytes32([Length32]byte{})

// Bytes returns a slice of bytes
func (h Bytes32) Bytes() []byte { return h[:] }

// Big returns the bytes as big integer
func (h Bytes32) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Equal checks equality between h and o
func (h Bytes32) Equal(o Bytes32) bool { return bytes.Equal(h.Bytes(), o.Bytes()) }

func (h Bytes32) String() string { return h.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (h Bytes32) HexStr() string {
	return ToHex(h[:])
}

// Hex encodes the bytes to hex
func (h Bytes32) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(dst, h[:])
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
func (h Bytes64) Bytes() []byte { return h[:] }

// Big returns the bytes as big integer
func (h Bytes64) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Equal checks equality between h and o
func (h Bytes64) Equal(o Bytes64) bool { return bytes.Equal(h.Bytes(), o.Bytes()) }

func (h Bytes64) String() string { return h.HexStr() }

// HexStr encodes the bytes to hex, prefixed with 0x
func (h Bytes64) HexStr() string {
	return ToHex(h[:])
}

// Hex encodes the bytes to hex
func (h Bytes64) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(dst, h[:])
	return dst
}

// IsEmpty checks whether the object is empty (having zero values)
func (h Bytes64) IsEmpty() bool {
	return h == EmptyBytes64
}

// HexToBytes64 creates an hex value to Bytes64
func HexToBytes64(hex string) (Bytes64, error) {
	bs, err := FromHex(hex)
	if err != nil {
		return EmptyBytes64, err
	}
	return BytesToBytes64(bs), nil
}

// BytesToBytes64 copies b to a Bytes64
func BytesToBytes64(b []byte) Bytes64 {
	var h Bytes64
	copy(h[:], b)
	return h
}

// StrToBytes64 converts a string to a Bytes64
func StrToBytes64(s string) Bytes64 {
	return BytesToBytes64([]byte(s))
}

// GenerateKeyPair generates private and public keys
func GenerateKeyPair(r io.Reader) (crypto.PrivKey, crypto.PubKey, error) {
	return crypto.GenerateEd25519Key(r)
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

// Decrypt decrypts a ciphertext
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

// RSAPubKeyID is like RSAPubKeyIDRaw except it returns hex encoded version
func RSAPubKeyID(pk *rsa.PublicKey) string {
	return ToHex(RSAPubKeyIDRaw(pk))
}

// RSAPubKeyIDRaw returns a 20 bytes fingerprint of the public key
func RSAPubKeyIDRaw(pk *rsa.PublicKey) []byte {
	return RIPEMD160(pk.N.Bytes())
}
