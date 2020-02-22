package core

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Common", func() {
	var key = crypto.NewKeyFromIntSeed(1)

	Describe("TxCommon.FromMap", func() {
		It("case 1", func() {
			sig := []byte("signature")
			ts := 123473664
			tx := &TxCommon{}
			err := tx.FromMap(map[string]interface{}{
				"nonce":        457736656,
				"fee":          1.2,
				"sig":          util.ToHex(sig),
				"timestamp":    ts,
				"senderPubKey": key.PubKey().Base58(),
			})
			Expect(err).To(BeNil())
			Expect(tx.Fee).To(Equal(util.String("1.2")))
			Expect(tx.Nonce).To(Equal(uint64(457736656)))
			Expect(tx.Sig).To(Equal(sig))
			Expect(tx.Timestamp).To(Equal(int64(ts)))
			Expect(tx.SenderPubKey.ToBytes32()).To(Equal(key.PubKey().MustBytes32()))
		})

		It("case 2 - 'string' nonce", func() {
			sig := []byte("signature")
			ts := 123473664
			tx := &TxCommon{}
			err := tx.FromMap(map[string]interface{}{
				"nonce":        "457736656",
				"fee":          1.2,
				"sig":          util.ToHex(sig),
				"timestamp":    ts,
				"senderPubKey": key.PubKey().Base58(),
			})
			Expect(err).To(BeNil())
			Expect(tx.Fee).To(Equal(util.String("1.2")))
			Expect(tx.Nonce).To(Equal(uint64(457736656)))
			Expect(tx.Sig).To(Equal(sig))
			Expect(tx.Timestamp).To(Equal(int64(ts)))
			Expect(tx.SenderPubKey.ToBytes32()).To(Equal(key.PubKey().MustBytes32()))
		})

		It("case 3 - 'int' fee", func() {
			sig := []byte("signature")
			ts := 123473664
			tx := &TxCommon{}
			err := tx.FromMap(map[string]interface{}{
				"nonce":        "457736656",
				"fee":          1,
				"sig":          util.ToHex(sig),
				"timestamp":    ts,
				"senderPubKey": key.PubKey().Base58(),
			})
			Expect(err).To(BeNil())
			Expect(tx.Fee).To(Equal(util.String("1")))
			Expect(tx.Nonce).To(Equal(uint64(457736656)))
			Expect(tx.Sig).To(Equal(sig))
			Expect(tx.Timestamp).To(Equal(int64(ts)))
			Expect(tx.SenderPubKey.ToBytes32()).To(Equal(key.PubKey().MustBytes32()))
		})

		It("case 3 - 'string' fee", func() {
			sig := []byte("signature")
			ts := 123473664
			tx := &TxCommon{}
			err := tx.FromMap(map[string]interface{}{
				"nonce":        "457736656",
				"fee":          "1",
				"sig":          util.ToHex(sig),
				"timestamp":    ts,
				"senderPubKey": key.PubKey().Base58(),
			})
			Expect(err).To(BeNil())
			Expect(tx.Fee).To(Equal(util.String("1")))
			Expect(tx.Nonce).To(Equal(uint64(457736656)))
			Expect(tx.Sig).To(Equal(sig))
			Expect(tx.Timestamp).To(Equal(int64(ts)))
			Expect(tx.SenderPubKey.ToBytes32()).To(Equal(key.PubKey().MustBytes32()))
		})

		It("case 4 - malformed hex-encoded signature", func() {
			tx := &TxCommon{}
			err := tx.FromMap(map[string]interface{}{
				"nonce":        "457736656",
				"fee":          "1",
				"sig":          "bad hex",
				"timestamp":    123473664,
				"senderPubKey": key.PubKey().Base58(),
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:sig, msg:unable to decode from hex"))
		})

		It("case 5 - malformed sender public key", func() {
			tx := &TxCommon{}
			err := tx.FromMap(map[string]interface{}{
				"nonce":        "457736656",
				"fee":          "1",
				"sig":          util.ToHex([]byte("signature")),
				"timestamp":    123473664,
				"senderPubKey": "bad public key",
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:senderPubKey, msg:unable to decode from base58"))
		})
	})
})
