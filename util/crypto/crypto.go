package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"io"

	"github.com/tendermint/tendermint/libs/bech32"
	"github.com/themakeos/lobe/types/constants"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/ripemd160"
)

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
	return hex.EncodeToString(Hash20(v))
}

// MakeNamespaceHash creates a hash of a namespace name
func MakeNamespaceHash(ns string) string {
	return Hash20Hex([]byte(ns))
}

// RIPEMD160 returns RIPEMD160 (20 bytes) hash of v
func RIPEMD160(v []byte) []byte {
	h := ripemd160.New()
	h.Write(v)
	return h.Sum(nil)
}

// MustDecodePushKeyID decodes a push key ID
// Panics if decoding fails.
func MustDecodePushKeyID(id string) []byte {
	_, bz, err := bech32.DecodeAndConvert(id)
	if err != nil {
		panic(err)
	}
	return bz
}

// IsValidPushAddr checks whether the given id is a valid bech32 encoded address
func IsValidPushAddr(id string) bool {
	hrp, bz, err := bech32.DecodeAndConvert(id)
	if err != nil {
		return false
	}
	if hrp != constants.PushAddrHRP {
		return false
	}
	if len(bz) != 20 {
		return false
	}
	return true
}
