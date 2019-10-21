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
				NewTx(TxTypeExecCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			}).ToNot(Panic())
		})
	})

	Describe("Tx.GetFrom", func() {
		It("should successfully get the sender address", func() {
			tx := NewTx(TxTypeExecCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			Expect(tx.GetFrom()).To(Equal(address.Addr()))
		})

		It("should panic if sender public key is invalid", func() {
			tx := NewTx(TxTypeExecCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			tx.SenderPubKey = util.String("invalid")
			Expect(func() {
				tx.GetFrom()
			}).To(Panic())
		})
	})

	Describe(".ToMap", func() {
		It("should successfully get the correct map equivalent", func() {
			tx := NewTx(TxTypeExecCoinTransfer, 0, "recipient_addr", address, "10", "0.1", 1)
			expected := map[string]interface{}{
				"to":           util.String("recipient_addr"),
				"senderPubKey": util.String("48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC"),
				"value":        util.String("10"),
				"timestamp":    int64(1),
				"sig": []uint8{
					22, 251, 175, 244, 34, 163, 137, 130, 237, 44, 187, 144, 52, 233, 138, 118,
					255, 212, 181, 92, 253, 203, 178, 92, 253, 204, 7, 100, 123, 238, 171, 125,
					17, 84, 31, 50, 190, 184, 131, 253, 68, 62, 49, 252, 161, 180, 163, 220, 44,
					113, 35, 180, 22, 49, 12, 1, 94, 12, 153, 121, 156, 75, 158, 2,
				},
				"type":           0,
				"previousSecret": []uint8{},
				"secret":         []uint8{},
				"fee":            util.String("0.1"),
				"nonce":          uint64(0x0000000000000000),
				"ticketID":       []uint8{},
			}
			Expect(tx.ToMap()).To(Equal(expected))
		})
	})

	Describe(".Bytes", func() {

		It("should return expected bytes", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			bs := tx.GetBytesNoSig()
			expected := []byte{155, 160, 207, 0, 0, 0, 0, 0, 0, 0, 1, 172, 115, 111, 109, 101, 95,
				112, 117, 98, 95, 107, 101, 121, 211, 0, 0, 0, 0, 0, 0, 0, 0, 172, 115, 111, 109,
				101, 95, 97, 100, 100, 114, 101, 115, 115, 211, 0, 0, 0, 0, 0, 0, 0, 1, 160, 192,
				192, 207, 0, 0, 0, 0, 0, 0, 0, 0, 192}
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
			expected := []byte{42, 156, 185, 145, 109, 109, 40, 219, 179, 38, 191, 84, 132, 205,
				137, 217, 2, 9, 123, 47, 9, 231, 246, 198, 254, 202, 4, 48, 201, 92, 189, 66, 254,
				35, 161, 101, 29, 137, 8, 216, 61, 45, 95, 61, 64, 19, 114, 210, 57, 119, 150,
				149, 160, 8, 182, 148, 162, 146, 202, 218, 248, 16, 233, 2}
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
			expected := util.BytesToHash([]byte{217, 252, 254, 135, 48, 92, 119, 119, 85, 94, 123,
				175, 44, 174, 52, 11, 143, 80, 118, 7, 113, 96, 179, 96, 167, 221, 45, 113, 187,
				133, 7, 132})
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
			Expect(tx.GetID()).To(Equal("0xd9fcfe87305c7777555e7baf2cae340b8f5076077160b360a7dd2d71bb850784"))
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
