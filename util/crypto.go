package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
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

// Bytes32 represents a hash value
type Bytes32 [Length32]byte

// EmptyHash is an empty Hash
var EmptyHash = Bytes32([Length32]byte{})

// Bytes gets the byte representation of the underlying hash.
func (h Bytes32) Bytes() []byte { return h[:] }

// Big converts a hash to a big integer.
func (h Bytes32) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Equal checks equality between h and o
func (h Bytes32) Equal(o Bytes32) bool { return bytes.Equal(h.Bytes(), o.Bytes()) }

func (h Bytes32) String() string { return h.HexStr() }

// HexStr returns the hex string version of the hash beginning with 0x
func (h Bytes32) HexStr() string {
	return ToHex(h[:])
}

// Hex is like HexStr but returns bytes
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

// IsEmpty checks whether the hash is empty (having zero values)
func (h Bytes32) IsEmpty() bool {
	return h == EmptyHash
}

// HexToHash creates an Hash from hex string
func HexToHash(hex string) (Bytes32, error) {
	bs, err := FromHex(hex)
	if err != nil {
		return EmptyHash, err
	}
	return BytesToHash(bs), nil
}

// BytesToHash copies b to a Hash
func BytesToHash(b []byte) Bytes32 {
	var h Bytes32
	copy(h[:], b)
	return h
}

// StrToHash converts a string to a Hash
func StrToHash(s string) Bytes32 {
	return BytesToHash([]byte(s))
}

// Bytes64 represents a 64 bytes value
type Bytes64 [Length64]byte

// EmptySig is an empty Hash
var EmptySig = Bytes64([Length64]byte{})

// Bytes gets the byte representation of the underlying hash.
func (s Bytes64) Bytes() []byte { return s[:] }

// Big converts a hash to a big integer.
func (s Bytes64) Big() *big.Int { return new(big.Int).SetBytes(s[:]) }

// Equal checks equality between h and o
func (s Bytes64) Equal(o Bytes64) bool { return bytes.Equal(s.Bytes(), o.Bytes()) }

func (s Bytes64) String() string { return s.HexStr() }

// HexStr returns the hex string version of the hash beginning with 0x
func (s Bytes64) HexStr() string {
	return ToHex(s[:])
}

// Hex is like HexStr but returns bytes
func (s Bytes64) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(s)))
	hex.Encode(dst, s[:])
	return dst
}

// SS returns a short version of HexStr with the middle
// characters truncated when length is at least 32
func (s Bytes64) SS() string {
	hexStr := s.HexStr()
	if len(s) >= 32 {
		return fmt.Sprintf("%s...%s", string(hexStr)[0:10], string(hexStr)[len(hexStr)-10:])
	}
	return hexStr
}

// IsEmpty checks whether the hash is empty (having zero values)
func (s Bytes64) IsEmpty() bool {
	return s == EmptySig
}

// HexToSig creates a Sig from hex string
func HexToSig(hex string) (Bytes64, error) {
	bs, err := FromHex(hex)
	if err != nil {
		return EmptySig, err
	}
	return BytesToSig(bs), nil
}

// BytesToSig copies b to a Sig
func BytesToSig(b []byte) Bytes64 {
	var s Bytes64
	copy(s[:], b)
	return s
}

// StrToSig converts a string to a Hash
func StrToSig(s string) Bytes64 {
	return BytesToSig([]byte(s))
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

// Sha1 returns sha1 hash of v
func Sha1(v []byte) []byte {
	h := sha1.New()
	h.Write(v)
	return h.Sum(nil)
}

// Sha1Hex returns sha1 in hex format
func Sha1Hex(v []byte) string {
	return fmt.Sprintf("%x", Sha1(v))
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
