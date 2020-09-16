package bdn

import (
	"bytes"
	"io"

	. "github.com/onsi/ginkgo"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"

	. "github.com/onsi/gomega"

	"github.com/make-os/lobe/util"
)

func readerFromBytes(bz []byte) *resetableReader {
	return &resetableReader{
		bz:  bz,
		buf: bytes.NewBuffer(bz),
	}
}

var _ = Describe("Bdn", func() {
	var rdr, rdr2 io.Reader

	BeforeEach(func() {
		rdr, rdr2 = readerFromBytes(util.RandBytes(32)), readerFromBytes(util.RandBytes(32))
	})

	Describe(".NewKey", func() {
		It("should generate valid private and public key", func() {
			priv, pub := NewKey(rdr)
			suite := bn256.NewSuite()
			priv2, pub2 := bdn.NewKeyPair(suite, random.New(rdr))
			Expect(priv.sk.Equal(priv2)).To(BeTrue())
			Expect(pub.pk.Equal(pub2)).To(BeTrue())
		})
	})

	Describe(".NewKeyFromSeed", func() {
		It("should generate valid private and public key", func() {
			priv, pub := NewKeyFromSeed(util.RandBytes(32))
			suite := bn256.NewSuite()
			priv2, pub2 := bdn.NewKeyPair(suite, random.New(rdr))
			Expect(priv.sk.Equal(priv2)).To(BeTrue())
			Expect(pub.pk.Equal(pub2)).To(BeTrue())
		})
	})

	Describe(".Bytes", func() {
		It("should return bytes of length 32 for private key and 128 for public key", func() {
			priv, pub := NewKey(rdr)
			Expect(priv.Bytes()).To(HaveLen(32))
			Expect(pub.Bytes()).To(HaveLen(128))
		})
	})

	Describe(".Sign & Verify", func() {
		Context("with valid signature", func() {
			Specify("that Verify returns nil", func() {
				priv, pub := NewKey(rdr)
				msg := []byte("message")
				sig, err := priv.Sign(msg)
				Expect(err).To(BeNil())
				Expect(sig).To(HaveLen(64))
				Expect(pub.Verify(sig, msg)).To(BeNil())
			})
		})

		Context("with invalid signature", func() {
			Specify("that Verify returns an error", func() {
				_, pub := NewKey(rdr)
				msg := []byte("message")
				sig := util.RandBytes(64)
				Expect(pub.Verify(sig, msg)).ToNot(BeNil())
			})
		})
	})

	Describe(".BytesToPublicKey", func() {
		It("should successfully convert to PublicKey", func() {
			_, pub := NewKey(rdr)
			bz := pub.Bytes()
			pk, err := BytesToPublicKey(bz)
			Expect(err).To(BeNil())
			Expect(pk.pk.Equal(pub.pk))
		})
	})

	Describe(".AggregateSignatures", func() {
		Context("with 2 public keys and 2 signatures", func() {
			It("should create an aggregated signature", func() {
				priv, pub := NewKey(rdr)
				priv2, pub2 := NewKey(rdr2)
				pubs := []*PublicKey{pub, pub2}

				msg := []byte("message")
				sig, _ := priv.Sign(msg)
				sig2, _ := priv2.Sign(msg)
				sigs := [][]byte{sig, sig2}

				aggBz, err := AggregateSignatures(pubs, sigs)
				Expect(err).To(BeNil())
				Expect(aggBz).To(HaveLen(64))
			})
		})
	})

	Describe(".AggregatePublicKeys", func() {
		Context("with 2 public keys", func() {
			It("should create an aggregated public key", func() {
				_, pub := NewKey(rdr)
				_, pub2 := NewKey(rdr2)
				pubs := []*PublicKey{pub, pub2}

				aggPubKey, err := AggregatePublicKeys(pubs)
				Expect(err).To(BeNil())
				Expect(aggPubKey).ToNot(BeNil())
			})
		})
	})
})
