package bls

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	kyber "go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"
)

type resetableReader struct {
	bz  []byte
	buf *bytes.Buffer
}

func (t *resetableReader) Read(p []byte) (n int, err error) {
	bz, err := ioutil.ReadAll(t.buf)
	if err != nil {
		return 0, err
	}
	t.buf.Reset()
	t.buf.Write(bz)
	return len(bz), nil
}

// PrivateKey represents a bn256 private key
type PrivateKey struct {
	sk    kyber.Scalar
	suite pairing.Suite
}

// NewKey creates a bn256 private and public key pair
func NewKey(reader io.Reader) (*PrivateKey, *PublicKey) {
	suite := bn256.NewSuite()
	privKey, _ := bdn.NewKeyPair(suite, random.New(reader))
	pk := &PrivateKey{privKey, suite}
	return pk, pk.Public()
}

// NewKeyFromSeed creates a bn256 private and public key pair from the given
// seed. Seed must be 32 bytes; If more, seed is truncated.
func NewKeyFromSeed(seed []byte) (*PrivateKey, *PublicKey) {
	rdr := &resetableReader{bz: seed, buf: bytes.NewBuffer(seed[:32])}
	return NewKey(rdr)
}

// Public return the corresponding public key
func (sk *PrivateKey) Public() *PublicKey {
	return &PublicKey{
		pk:    sk.suite.G2().Point().Mul(sk.sk, nil),
		suite: sk.suite,
	}
}

// Bytes returns the binary form of the key
func (sk *PrivateKey) Bytes() []byte {
	data, _ := sk.sk.MarshalBinary()
	return data
}

// Sign creates a BLS signature on the given message
func (sk *PrivateKey) Sign(msg []byte) ([]byte, error) {
	return bdn.Sign(sk.suite, sk.sk, msg)
}

// PublicKey represents a bn256 public key
type PublicKey struct {
	pk    kyber.Point
	suite pairing.Suite
}

// Bytes returns the binary form of the key
func (pk *PublicKey) Bytes() []byte {
	data, _ := pk.pk.MarshalBinary()
	return data
}

// Verify checks the given BLS signature on the message
func (pk *PublicKey) Verify(sig, msg []byte) error {
	return bdn.Verify(pk.suite, pk.pk, msg, sig)
}

// BytesToPublicKey converts the byte slice of a marshaled public key to a
// PublicKey object
func BytesToPublicKey(bz []byte) (*PublicKey, error) {
	suite := bn256.NewSuite()
	point := suite.G2().Point()
	if err := point.UnmarshalBinary(bz); err != nil {
		return nil, err
	}
	return &PublicKey{
		suite: suite,
		pk:    point,
	}, nil
}

// AggregateSignatures takes a slice of PublicKeys and their corresponding signatures to
// create an aggregate signature
func AggregateSignatures(pubKeys []*PublicKey, sig [][]byte) ([]byte, error) {

	publicKeys := []kyber.Point{}
	for _, pk := range pubKeys {
		publicKeys = append(publicKeys, pk.pk)
	}

	suite := bn256.NewSuite()
	mask, _ := sign.NewMask(suite, publicKeys, nil)
	for i := range publicKeys {
		mask.SetBit(i, true)
	}

	aggSig, err := bdn.AggregateSignatures(suite, sig, mask)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signature aggregate")
	}

	aggSigBz, err := aggSig.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return aggSigBz, nil
}

// AggregatePublicKeys takes a slice of PublicKeys to create an
// aggregate public key
func AggregatePublicKeys(pubKeys []*PublicKey) (*PublicKey, error) {

	publicKeys := []kyber.Point{}
	for _, pk := range pubKeys {
		publicKeys = append(publicKeys, pk.pk)
	}

	suite := bn256.NewSuite()
	mask, _ := sign.NewMask(suite, publicKeys, nil)
	for i := range publicKeys {
		mask.SetBit(i, true)
	}

	aggPubKey, err := bdn.AggregatePublicKeys(suite, mask)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create public key aggregate")
	}

	return &PublicKey{pk: aggPubKey, suite: suite}, nil
}
