package types

import (
	"time"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transaction", func() {

	var address = crypto.NewKeyFromIntSeed(1)

	Describe(".NewTx", func() {
		It("should successfully create and sign a new transaction", func() {
			Expect(func() {
				NewTx(TxTypeCoin, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			}).ToNot(Panic())
		})
	})

	Describe(".Bytes", func() {

		It("should return expected bytes", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			bs := tx.GetBytesNoHashAndSig()
			expected := []byte{151, 160, 207, 0, 0, 0, 0, 0, 0, 0, 1, 172, 115, 111, 109, 101, 95,
				112, 117, 98, 95, 107, 101, 121, 211, 0, 0, 0, 0, 0, 0, 0, 0, 172, 115, 111, 109,
				101, 95, 97, 100, 100, 114, 101, 115, 115, 211, 0, 0, 0, 0, 0, 0, 0, 1, 160}
			Expect(bs).ToNot(BeEmpty())
			Expect(bs).To(Equal(expected))
		})
	})

	Describe(".NewTxFromBytes", func() {
		It("should", func() {
			tx := &Transaction{Type: 1, Nonce: 1, Timestamp: 1000, Fee: "2", Sig: []byte("sig"), Value: "10", To: "some_address", SenderPubKey: "some_pub_key"}
			decTx, err := NewTxFromBytes(tx.Bytes())
			Expect(err).To(BeNil())
			Expect(tx).To(Equal(decTx))
		})
	})

	Describe(".TxSign", func() {
		It("should return error = 'nil tx' when tx is nil", func() {
			_, err := TxSign(nil, "private key")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("nil tx"))
		})

		It("should expected signature", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			sig, err := TxSign(tx, a.PrivKey().Base58())
			expected := []byte{133, 59, 147, 97, 186, 226, 238, 250, 152, 81, 162, 242, 48, 237,
				255, 253, 101, 55, 118, 179, 177, 202, 21, 110, 36, 227, 42, 253, 14, 208, 201,
				13, 224, 69, 250, 126, 1, 47, 144, 92, 74, 13, 253, 56, 65, 131, 252, 185, 0, 126,
				219, 104, 27, 96, 14, 101, 73, 137, 95, 23, 246, 91, 110, 0}
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())
			Expect(sig).To(Equal(expected))
			Expect(sig).To(HaveLen(64))
		})
	})

	Describe(".TxVerify", func() {
		It("should return error = 'nil tx' when nil is passed", func() {
			err := TxVerify(nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("nil tx"))
		})

		It("should return err = 'sender public not set' when sender public key is not set on "+
			"the transaction", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address"}
			err := TxVerify(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public not set"))
		})

		It("should return err = 'signature not set' when signature is unset", func() {
			tx := &Transaction{Type: 1, Nonce: 1, SenderPubKey: "pub key", To: "some_address"}
			err := TxVerify(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:sig, error:signature not set"))
		})

		It("should return error if senderPubKey is invalid", func() {
			tx := &Transaction{Type: 1, Nonce: 1, SenderPubKey: "pub key", To: "some_address",
				Sig: []byte("some_sig")}
			err := TxVerify(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:senderPubKey, error:invalid format: version " +
				"and/or checksum bytes missing"))
		})

		It("should verify successfully", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address",
				SenderPubKey: util.String(a.PubKey().Base58())}
			sig, err := TxSign(tx, a.PrivKey().Base58())
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())

			tx.Sig = sig
			err = TxVerify(tx)
			Expect(err).To(BeNil())
		})

		It("should return err = 'verify failed' when verification failed", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address",
				SenderPubKey: util.String(a.PubKey().Base58())}
			sig, err := TxSign(tx, a.PrivKey().Base58())
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())

			tx.Sig = sig
			tx.To = "altered_address"
			err = TxVerify(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrTxVerificationFailed))
		})
	})

	Describe(".ComputeHash", func() {
		It("should successfully return hash", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address",
				SenderPubKey: util.String(a.PubKey().Base58())}
			expected := util.BytesToHash([]byte{165, 1, 247, 216, 79, 179, 10, 143, 14, 4, 111, 71,
				68, 4, 198, 20, 151, 37, 90, 95, 180, 228, 25, 125, 116, 197, 203, 232, 178,
				116, 85, 14})
			Expect(tx.ComputeHash()).To(Equal(expected))
		})
	})

	Describe(".ID", func() {
		It("should return expected transaction ID", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address",
				SenderPubKey: util.String(a.PubKey().Base58())}
			Expect(tx.GetID()).
				To(Equal("0xa501f7d84fb30a8f0e046f474404c61497255a5fb4e4197d74c5cbe8b274550e"))
		})
	})
})
