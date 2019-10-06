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
			expected := map[string]interface{}{
				"to":           util.String("recipient_addr"),
				"senderPubKey": util.String("48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC"),
				"value":        util.String("10"),
				"timestamp":    int64(1),
				"sig": []uint8{
					84, 220, 181, 130, 68, 216, 132, 241, 106, 140, 135, 54, 96, 96, 162, 225, 124,
					7, 243, 126, 9, 57, 212, 199, 28, 43, 43, 199, 166, 70, 71, 159, 104, 160, 24,
					173, 171, 192, 42, 223, 31, 25, 76, 237, 248, 192, 134, 119, 34, 192, 192, 0,
					18, 178, 226, 163, 191, 171, 63, 85, 69, 186, 175, 12,
				},
				"type":           0,
				"secretRound":    uint64(0),
				"previousSecret": []uint8{},
				"secret":         []uint8{},
				"fee":            util.String("0.1"),
				"nonce":          uint64(0x0000000000000000),
			}
			Expect(tx.ToMap()).To(Equal(expected))
		})
	})

	Describe(".Bytes", func() {

		It("should return expected bytes", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			bs := tx.GetBytesNoSig()
			expected := []byte{154, 160, 207, 0, 0, 0, 0, 0, 0, 0, 1, 172, 115, 111, 109, 101, 95,
				112, 117, 98, 95, 107, 101, 121, 211, 0, 0, 0, 0, 0, 0, 0, 0, 172, 115, 111, 109,
				101, 95, 97, 100, 100, 114, 101, 115, 115, 211, 0, 0, 0, 0, 0, 0, 0, 1, 160, 192,
				192, 207, 0, 0, 0, 0, 0, 0, 0, 0}
			Expect(bs).ToNot(BeEmpty())
			Expect(bs).To(Equal(expected))
		})
	})

	Describe(".NewTxFromBytes", func() {
		It("should", func() {
			tx := NewBareTx(1)
			decTx, err := NewTxFromBytes(tx.Bytes())
			Expect(err).To(BeNil())
			Expect(tx).To(Equal(decTx))
		})
	})

	Describe(".SignTx", func() {
		It("should return error = 'nil tx' when tx is nil", func() {
			_, err := SignTx(nil, "private key")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("nil tx"))
		})

		It("should expected signature", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			sig, err := SignTx(tx, a.PrivKey().Base58())
			expected := []byte{249, 244, 249, 139, 220, 37, 1, 207, 232, 69, 246, 226, 71, 129,
				128, 54, 211, 84, 218, 79, 48, 253, 43, 124, 69, 83, 14, 216, 118, 145, 144,
				158, 98, 143, 82, 171, 215, 0, 246, 149, 247, 221, 189, 246, 238, 155, 165,
				206, 176, 105, 19, 183, 0, 136, 197, 208, 2, 53, 212, 183, 64, 83, 173, 7}
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())
			Expect(sig).To(Equal(expected))
			Expect(sig).To(HaveLen(64))
		})
	})

	Describe(".VerifyTx", func() {
		It("should return error = 'nil tx' when nil is passed", func() {
			err := VerifyTx(nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("nil tx"))
		})

		It("should return err = 'sender public not set' when sender public key is not set on "+
			"the transaction", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address"}
			err := VerifyTx(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public not set"))
		})

		It("should return err = 'signature not set' when signature is unset", func() {
			tx := &Transaction{Type: 1, Nonce: 1, SenderPubKey: "pub key", To: "some_address"}
			err := VerifyTx(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:sig, error:signature not set"))
		})

		It("should return error if senderPubKey is invalid", func() {
			tx := &Transaction{Type: 1, Nonce: 1, SenderPubKey: "pub key", To: "some_address",
				Sig: []byte("some_sig")}
			err := VerifyTx(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:senderPubKey, error:invalid format: version " +
				"and/or checksum bytes missing"))
		})

		It("should verify successfully", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address",
				SenderPubKey: util.String(a.PubKey().Base58())}
			sig, err := SignTx(tx, a.PrivKey().Base58())
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())

			tx.Sig = sig
			err = VerifyTx(tx)
			Expect(err).To(BeNil())
		})

		It("should return err = 'verify failed' when verification failed", func() {
			seed := int64(1)
			a, _ := crypto.NewKey(&seed)
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address",
				SenderPubKey: util.String(a.PubKey().Base58())}
			sig, err := SignTx(tx, a.PrivKey().Base58())
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())

			tx.Sig = sig
			tx.To = "altered_address"
			err = VerifyTx(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrTxVerificationFailed))
		})
	})

	Describe(".ComputeHash", func() {
		It("should successfully return hash", func() {
			sender := crypto.NewKeyFromIntSeed(1)
			tx := NewBareTx(1)
			tx.Timestamp = 1
			tx.SetSenderPubKey(util.String(sender.PubKey().Base58()))
			tx.To = "address_1"
			expected := util.BytesToHash([]byte{91, 116, 169, 35, 61, 144, 94, 99, 83, 131, 43,
				68, 67, 86, 28, 142, 243, 140, 183, 35, 75, 214, 16, 80, 11, 163, 46, 254, 11,
				255, 137, 199})
			Expect(tx.ComputeHash()).To(Equal(expected))
		})
	})

	Describe(".GetID", func() {
		It("should return expected transaction ID", func() {
			sender := crypto.NewKeyFromIntSeed(1)
			tx := NewBareTx(1)
			tx.Timestamp = 1
			tx.SetSenderPubKey(util.String(sender.PubKey().Base58()))
			tx.To = "address_1"
			Expect(tx.GetID()).To(Equal("0x5b74a9233d905e6353832b4443561c8ef38cb7234bd610500ba32efe0bff89c7"))
		})
	})

	Describe(".GetMeta", func() {
		It("should return the tx meta", func() {
			tx := NewBareTx(1)
			meta := tx.GetMeta()
			meta[TxMetaKeyInvalidated] = struct{}{}
			Expect(tx.GetMeta()).To(Equal(meta))
		})
	})

	Describe(".IsInvalidated", func() {
		It("should return true when meta has an invalidation key", func() {
			tx := NewBareTx(1)
			res := tx.IsInvalidated()
			Expect(res).To(BeFalse())
			meta := tx.GetMeta()
			meta[TxMetaKeyInvalidated] = struct{}{}
			res = tx.IsInvalidated()
			Expect(res).To(BeTrue())
		})
	})

	Describe(".Invalidate", func() {
		It("should set TxMetaKeyInvalidated in the tx meta", func() {
			tx := NewBareTx(1)
			tx.Invalidate()
			Expect(tx.GetMeta()).To(HaveKey(TxMetaKeyInvalidated))
		})
	})
})
