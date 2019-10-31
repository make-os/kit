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
					0x14, 0x77, 0x79, 0x0f, 0x25, 0xa2, 0x04, 0x75, 0x59, 0xb6, 0x84, 0xe0, 0xb4, 0x9a, 0xf6, 0xf8,
					0xee, 0xd3, 0x6a, 0x20, 0x2f, 0x40, 0xd9, 0x28, 0xa9, 0x79, 0x8b, 0xda, 0x06, 0x0f, 0xc2, 0x71,
					0x21, 0x10, 0x95, 0xa6, 0x69, 0x92, 0xd1, 0x6f, 0x3d, 0xd6, 0xc4, 0xed, 0xa0, 0x92, 0x3e, 0xf1,
					0xcf, 0xb4, 0xa8, 0x53, 0xed, 0x3f, 0x85, 0x87, 0xe5, 0x50, 0xfa, 0x0c, 0x7b, 0xca, 0x28, 0x07,
				},
				"type":  0,
				"fee":   util.String("0.1"),
				"nonce": uint64(0x0000000000000000),
			}
			Expect(tx.ToMap()).To(Equal(expected))
		})
	})

	Describe(".Bytes", func() {

		It("should return expected bytes", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			bs := tx.GetBytesNoSig()
			expected := []byte{153, 160, 207, 0, 0, 0, 0, 0, 0, 0, 1, 172, 115, 111, 109, 101, 95, 112, 117, 98, 95, 107, 101, 121, 211, 0, 0, 0, 0, 0, 0, 0, 0, 172, 115, 111, 109, 101, 95, 97, 100, 100, 114, 101, 115, 115, 211, 0, 0, 0, 0, 0, 0, 0, 1, 160, 192, 192}
			Expect(bs).ToNot(BeEmpty())
			Expect(bs).To(Equal(expected))
		})
	})

	Describe(".NewTxFromBytes", func() {
		It("case 1", func() {
			tx := NewBareTx(1)
			decTx, err := NewTxFromBytes(tx.Bytes())
			Expect(err).To(BeNil())
			Expect(tx).To(Equal(decTx))
		})

		It("case 2 - with EpochSecret not nil", func() {
			tx := NewBareTx(1)
			tx.EpochSecret = &EpochSecret{}
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
			expected := []byte{203, 34, 205, 183, 7, 223, 56, 51, 205, 220, 127, 251, 152, 81, 195, 10, 28, 69, 216, 158, 173, 154, 177, 209, 213, 71, 65, 251, 168, 6, 61, 207, 90, 205, 216, 247, 69, 202, 194, 16, 97, 135, 46, 193, 253, 228, 14, 170, 230, 42, 168, 140, 21, 157, 201, 124, 179, 10, 111, 104, 6, 248, 31, 10}
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
			expected := util.BytesToHash([]byte{211, 88, 40, 205, 16, 123, 77, 20, 177, 108, 116, 100, 82, 0, 39, 181, 4, 239, 156, 161, 60, 221, 255, 217, 23, 166, 112, 238, 245, 70, 71, 96})
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
			Expect(tx.GetID()).To(Equal("0xd35828cd107b4d14b16c7464520027b504ef9ca13cddffd917a670eef5464760"))
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

	Describe("EpochSecret.Bytes", func() {
		It("should return expected bytes", func() {
			es := &EpochSecret{Secret: []byte("secret"), PreviousSecret: []byte("p_secret"), SecretRound: 1}
			bz := es.Bytes()
			Expect(bz).To(Equal([]byte{0x93, 0xc4, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0xcf, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xc4, 0x08, 0x70, 0x5f, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74}))
		})
	})

	Describe("EpochSecret.EpochSecretFromBytes", func() {
		Context("with empty argument", func() {
			It("should return empty EpochSecret", func() {
				es := EpochSecretFromBytes([]byte{})
				Expect(es).To(Equal(&EpochSecret{}))
			})
		})

		Context("with invalid/malformed bytes", func() {
			It("should panic", func() {
				Expect(func() {
					EpochSecretFromBytes([]byte{1, 2, 3})
				}).To(Panic())
			})
		})

		Context("with valid bytes", func() {
			bz := []byte{0x93, 0xc4, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0xcf, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xc4, 0x08, 0x70, 0x5f, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74}
			es := &EpochSecret{Secret: []byte("secret"), PreviousSecret: []byte("p_secret"), SecretRound: 1}
			It("should return expected EpochSecret object", func() {
				res := EpochSecretFromBytes(bz)
				Expect(res).To(Equal(es))
			})
		})
	})
})
