// Package crypto provides key and address creation functions.
package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"

	"github.com/libp2p/go-libp2p-core/crypto"
	pb "github.com/libp2p/go-libp2p-core/crypto/pb"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ellcrys/go-ethereum/crypto/sha3"
	"github.com/gogo/protobuf/proto"
	"github.com/makeos/mosdef/util"
	"golang.org/x/crypto/ripemd160"

	"github.com/libp2p/go-libp2p-core/peer"
)

// AddressVersion is the base58 encode version adopted
var AddressVersion byte = 92

// PublicKeyVersion is the base58 encode version adopted for public keys
var PublicKeyVersion byte = 93

// PrivateKeyVersion is the base58 encode version adopted for private keys
var PrivateKeyVersion byte = 94

// Key includes a wrapped Ed25519 private key and
// convenient methods to get the corresponding public
// key and transaction address.
type Key struct {
	privKey *PrivKey
	Meta    map[string]interface{}
}

// PubKey represents a public key
type PubKey struct {
	pubKey crypto.PubKey
}

// PrivKey represents a private key
type PrivKey struct {
	privKey crypto.PrivKey
}

// NewKey creates a new Ed25519 key
func NewKey(seed *int64) (*Key, error) {

	var r = rand.Reader

	if seed != nil {
		r = mrand.New(mrand.NewSource(*seed))
	}

	priv, _, err := crypto.GenerateEd25519Key(r)
	if err != nil {
		return nil, err
	}

	return &Key{
		privKey: &PrivKey{privKey: priv},
		Meta:    make(map[string]interface{}),
	}, nil
}

// NewKeyFromIntSeed is like NewKey but accepts seed of
// type Int and casts to Int64.
func NewKeyFromIntSeed(seed int) *Key {
	int64Seed := int64(seed)
	key, _ := NewKey(&int64Seed)
	return key
}

// NewKeyFromPrivKey creates a new Key instance from a PrivKey
func NewKeyFromPrivKey(sk *PrivKey) *Key {
	return &Key{privKey: sk}
}

// idFromPublicKey derives the libp2p peer ID from an Ed25519 public key
func idFromPublicKey(pk crypto.PubKey) (string, error) {
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// PeerID returns the IPFS compatible peer ID
func (k *Key) PeerID() string {
	pid, _ := idFromPublicKey(k.PubKey().pubKey)
	return pid
}

// Addr returns the transaction address
func (k *Key) Addr() util.String {
	return k.PubKey().Addr()
}

// PubKey returns the public key
func (k *Key) PubKey() *PubKey {
	return &PubKey{
		pubKey: k.privKey.privKey.GetPublic(),
	}
}

// PrivKey returns the private key
func (k *Key) PrivKey() *PrivKey {
	return k.privKey
}

// Bytes returns the byte equivalent of the public key
func (p *PubKey) Bytes() ([]byte, error) {
	if p.pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return p.pubKey.(*crypto.Ed25519PublicKey).Raw()
}

// MustBytes is like Bytes but panics on error
func (p *PubKey) MustBytes() []byte {
	if p.pubKey == nil {
		panic(fmt.Errorf("public key is nil"))
	}
	bz, err := p.pubKey.(*crypto.Ed25519PublicKey).Raw()
	if err != nil {
		panic(err)
	}
	return bz
}

// MustBytes32 is like Bytes but panics on error
func (p *PubKey) MustBytes32() util.Bytes32 {
	if p.pubKey == nil {
		panic(fmt.Errorf("public key is nil"))
	}
	bz, err := p.pubKey.(*crypto.Ed25519PublicKey).Raw()
	if err != nil {
		panic(err)
	}
	return util.BytesToBytes32(bz)
}

// Hex returns the public key in hex encoding
func (p *PubKey) Hex() string {
	bs, _ := p.Bytes()
	return hex.EncodeToString(bs)
}

// Base58 returns the public key in base58 encoding
func (p *PubKey) Base58() string {
	bs, _ := p.Bytes()
	return base58.CheckEncode(bs, PublicKeyVersion)
}

// Verify verifies a signature
func (p *PubKey) Verify(data, sig []byte) (bool, error) {
	return p.pubKey.Verify(data, sig)
}

// Addr computes an address from the public key
func (p *PubKey) Addr() util.String {
	pk, _ := p.Bytes()

	// Step 1: sha3 hash public key
	pubSha256 := sha3.Sum256(pk)

	// Step 2: RIPEMD160 hash the sha3 output
	r := ripemd160.New()
	r.Write(pubSha256[:])
	addr := r.Sum(nil)

	// Step 3: Base58 checksum step2 output
	var addr20 [20]byte
	copy(addr20[:], addr[:])
	return RIPEMD160ToAddr(addr20)
}

// Bytes returns the byte equivalent of the public key
func (p *PrivKey) Bytes() ([]byte, error) {
	if p.privKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return p.privKey.(*crypto.Ed25519PrivateKey).Raw()
}

// Bytes64 is like Bytes but returns util.Bytes64
func (p *PrivKey) Bytes64() (util.Bytes64, error) {
	bz, err := p.Bytes()
	if err != nil {
		return util.EmptyBytes64, nil
	}
	return util.BytesToBytes64(bz), nil
}

// Marshal encodes the private key using protocol buffer
func (p *PrivKey) Marshal() ([]byte, error) {
	pbmes := new(pb.PrivateKey)
	typ := p.privKey.Type()
	pbmes.Type = typ
	data, err := p.privKey.Raw()
	if err != nil {
		return nil, err
	}

	pbmes.Data = data
	return proto.Marshal(pbmes)
}

// Base58 returns the public key in base58 encoding
func (p *PrivKey) Base58() string {
	bs, _ := p.Bytes()
	return base58.CheckEncode(bs, PrivateKeyVersion)
}

// Sign signs a message
func (p *PrivKey) Sign(data []byte) ([]byte, error) {
	return p.privKey.Sign(data)
}

// MustSign signs a message but panics if error occurs
func (p *PrivKey) MustSign(data []byte) []byte {
	sig, err := p.privKey.Sign(data)
	if err != nil {
		panic(err)
	}
	return sig
}

// Key returns the wrapped crypto.PrivKey
func (p *PrivKey) Key() crypto.PrivKey {
	return p.privKey
}

// IsValidAddr checks whether an address is valid
func IsValidAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}

	result, v, err := base58.CheckDecode(addr)
	if err != nil {
		return err
	}

	if len(result) != 20 {
		return fmt.Errorf("invalid address size")
	}

	if v != AddressVersion {
		return fmt.Errorf("invalid version")
	}

	return nil
}

// RIPEMD160ToAddr returns a 20 byte slice to an address
func RIPEMD160ToAddr(hash [20]byte) util.String {
	return util.String(base58.CheckEncode(hash[:], AddressVersion))
}

// DecodeAddr validates an address, decodes it and returns
// raw encoded 20-bytes address
func DecodeAddr(addr string) ([20]byte, error) {

	var b [20]byte

	if err := IsValidAddr(addr); err != nil {
		return b, err
	}

	result, _, err := base58.CheckDecode(addr)
	if err != nil {
		return b, err
	}

	copy(b[:], result)

	return b, nil
}

// DecodeAddrOnly is like DecodeAddr except it does not validate the address
func DecodeAddrOnly(addr string) ([20]byte, error) {

	var b [20]byte

	result, _, err := base58.CheckDecode(addr)
	if err != nil {
		return b, err
	}

	copy(b[:], result)

	return b, nil
}

// IsValidPubKey checks whether a public key is valid
func IsValidPubKey(pubKey string) error {

	if pubKey == "" {
		return fmt.Errorf("empty pub key")
	}

	_, v, err := base58.CheckDecode(pubKey)
	if err != nil {
		return err
	}

	if v != PublicKeyVersion {
		return fmt.Errorf("invalid version")
	}

	return nil
}

// IsValidPrivKey checks whether a private key is valid
func IsValidPrivKey(privKey string) error {

	if privKey == "" {
		return fmt.Errorf("empty priv key")
	}

	_, v, err := base58.CheckDecode(privKey)
	if err != nil {
		return err
	}

	if v != PrivateKeyVersion {
		return fmt.Errorf("invalid version")
	}

	return nil
}

// PubKeyFromBase58 decodes a base58 encoded public key
func PubKeyFromBase58(pk string) (*PubKey, error) {

	if err := IsValidPubKey(pk); err != nil {
		return nil, err
	}

	decPubKey, _, _ := base58.CheckDecode(pk)
	pubKey, err := crypto.UnmarshalEd25519PublicKey(decPubKey)
	if err != nil {
		return nil, err
	}

	return &PubKey{
		pubKey: pubKey,
	}, nil
}

// PubKeyFromBytes returns a PubKey instance from a 32 bytes public key
func PubKeyFromBytes(pk []byte) (*PubKey, error) {
	pubKey, err := crypto.UnmarshalEd25519PublicKey(pk)
	if err != nil {
		return nil, err
	}

	return &PubKey{
		pubKey: pubKey,
	}, nil
}

// MustPubKeyFromBytes is like PubKeyFromBytes, except it panics if pk is invalid
func MustPubKeyFromBytes(pk []byte) *PubKey {
	pubKey, err := crypto.UnmarshalEd25519PublicKey(pk)
	if err != nil {
		panic(err)
	}

	return &PubKey{
		pubKey: pubKey,
	}
}

// MustBase58FromPubKeyBytes takes a raw public key bytes and returns its base58
// encoded version. It panics if pk is invalid
func MustBase58FromPubKeyBytes(pk []byte) string {
	pubKey, err := crypto.UnmarshalEd25519PublicKey(pk)
	if err != nil {
		panic(err)
	}

	wrapped := PubKey{pubKey: pubKey}
	return wrapped.Base58()
}

// PrivKeyFromBase58 decodes a base58 encoded private key
func PrivKeyFromBase58(pk string) (*PrivKey, error) {

	if err := IsValidPrivKey(pk); err != nil {
		return nil, err
	}

	sk, _, _ := base58.CheckDecode(pk)
	privKey, err := crypto.UnmarshalEd25519PrivateKey(sk)
	if err != nil {
		return nil, err
	}

	return &PrivKey{
		privKey: privKey,
	}, nil
}

// PrivKeyFromBytes returns a PrivKey instance from a 64 bytes private key
func PrivKeyFromBytes(bz [64]byte) (*PrivKey, error) {
	privKey, err := crypto.UnmarshalEd25519PrivateKey(bz[:])
	if err != nil {
		return nil, err
	}

	return &PrivKey{
		privKey: privKey,
	}, nil
}

// PrivKeyFromTMPrivateKey encodes a tendermint private key to PrivKey
func PrivKeyFromTMPrivateKey(tmSk ed25519.PrivKeyEd25519) (*PrivKey, error) {
	return PrivKeyFromBytes(tmSk)
}

// TMPubKeyFromBase58PubKey encodes a base58 encoded ed25519 public key to
// tendermint's ed25519.PubKeyEd25519
func TMPubKeyFromBase58PubKey(b58PubKey string) (ed25519.PubKeyEd25519, error) {

	var pubKeySized = [ed25519.PubKeyEd25519Size]byte{}

	pubKey, err := PubKeyFromBase58(b58PubKey)
	if err != nil {
		return pubKeySized, err
	}
	rawPubKey, _ := pubKey.Bytes()
	copy(pubKeySized[:], rawPubKey)

	return pubKeySized, nil
}

// TMPubKeyFromBytesPubKey is like TMPubKeyFromBase58PubKey but takes a byte slice
func TMPubKeyFromBytesPubKey(bzPubKey []byte) (ed25519.PubKeyEd25519, error) {

	var pubKeySized = [ed25519.PubKeyEd25519Size]byte{}

	pubKey, err := PubKeyFromBytes(bzPubKey)
	if err != nil {
		return pubKeySized, err
	}
	rawPubKey, _ := pubKey.Bytes()
	copy(pubKeySized[:], rawPubKey)

	return pubKeySized, nil
}
