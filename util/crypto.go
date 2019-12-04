package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/ripemd160"

	"golang.org/x/crypto/blake2b"

	crypto "github.com/libp2p/go-libp2p-crypto"
)

const (
	// HashLength is the standard size of hash values
	HashLength = 32
)

// Hash represents a hash value
type Hash [HashLength]byte

// EmptyHash is an empty Hash
var EmptyHash = Hash([HashLength]byte{})

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Big converts a hash to a big integer.
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Equal checks equality between h and o
func (h Hash) Equal(o Hash) bool { return bytes.Equal(h.Bytes(), o.Bytes()) }

func (h Hash) String() string { return h.HexStr() }

// HexStr returns the hex string version of the hash beginning with 0x
func (h Hash) HexStr() string {
	return ToHex(h[:])
}

// Hex is like HexStr but returns bytes
func (h Hash) Hex() []byte {
	dst := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(dst, h[:])
	return dst
}

// SS returns a short version of HexStr with the middle
// characters truncated when length is at least 32
func (h Hash) SS() string {
	s := h.HexStr()
	if len(s) >= 32 {
		return fmt.Sprintf("%s...%s", string(s)[0:10], string(s)[len(s)-10:])
	}
	return s
}

// IsEmpty checks whether the hash is empty (having zero values)
func (h Hash) IsEmpty() bool {
	return h == EmptyHash
}

// HexToHash creates an Hash from hex string
func HexToHash(hex string) (Hash, error) {
	bs, err := FromHex(hex)
	if err != nil {
		return EmptyHash, err
	}
	return BytesToHash(bs), nil
}

// BytesToHash copies b to a Hash
func BytesToHash(b []byte) Hash {
	var h Hash
	copy(h[:], b)
	return h
}

// StrToHash converts a string to a Hash
func StrToHash(s string) Hash {
	return BytesToHash([]byte(s))
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
