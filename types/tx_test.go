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
				NewTx(TxTypeCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			}).ToNot(Panic())
		})
	})

	Describe("Tx.GetFrom", func() {
		It("should successfully get the sender address", func() {
			tx := NewTx(TxTypeCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			Expect(tx.GetFrom()).To(Equal(address.Addr()))
		})

		It("should panic if sender public key is invalid", func() {
			tx := NewTx(TxTypeCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			tx.SenderPubKey = util.String("invalid")
			Expect(func() {
				tx.GetFrom()
			}).To(Panic())
		})
	})

	Describe(".ToMap", func() {
		It("should successfully get the correct map equivalent", func() {
			tx := NewTx(TxTypeCoinTransfer, 0, "recipient_addr", address, "10", "0.1", 1)
			Expect(tx.ToMap()).To(Equal(map[string]interface{}{
				"to":           util.String("recipient_addr"),
				"senderPubKey": util.String("48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC"),
				"value":        util.String("10"),
				"timestamp":    int64(1),
				"sig": []uint8{
					0x00, 0xa4, 0x0b, 0x21, 0xff, 0xd9, 0x8e, 0x8f, 0x3f, 0xe9, 0x6a, 0x92, 0x1b, 0x5c, 0x66, 0xbb,
					0x39, 0x07, 0x09, 0x13, 0xee, 0x0f, 0x09, 0xb0, 0x88, 0xe2, 0xc5, 0xaf, 0x06, 0x7f, 0x74, 0x8b,
					0xa9, 0xd0, 0xb8, 0x62, 0x7b, 0xc3, 0x2c, 0xcc, 0xe7, 0xdc, 0x5a, 0xe8, 0x74, 0x88, 0x1d, 0x20,
					0x67, 0x54, 0x72, 0x81, 0x0b, 0x52, 0x30, 0x16, 0x55, 0x21, 0x19, 0xe9, 0xf2, 0x32, 0x25, 0x0a,
				},
				"type": 0,
				"hash": util.Hash{
					0x48, 0x1e, 0xd7, 0x60, 0x1f, 0x7f, 0x02, 0x8e, 0x13, 0x6a, 0xb8, 0x0c, 0x21, 0x41, 0x55, 0x61,
					0x79, 0xd4, 0xe7, 0x49, 0xfb, 0xf6, 0x93, 0xde, 0x01, 0xfd, 0x47, 0xd8, 0x05, 0x71, 0xf1, 0x74,
				},
				"fee":   util.String("0.1"),
				"nonce": uint64(0x0000000000000000),
			}))
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
