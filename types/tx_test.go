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
					146, 40, 39, 100, 251, 150, 166, 38, 184, 171, 115, 192, 20, 123, 211, 251, 187, 235, 148, 232, 248, 33, 248, 16, 118, 126, 0, 189, 216, 159, 37, 7, 208, 29, 208, 132, 225, 104, 164, 71, 226, 96, 245, 49, 205, 69, 243, 65, 65, 105, 185, 232, 7, 50, 100, 249, 225, 192, 14, 232, 143, 145, 57, 6,
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
			expected := []byte{154, 160, 207, 0, 0, 0, 0, 0, 0, 0, 1, 172, 115, 111, 109, 101, 95, 112, 117, 98, 95, 107, 101, 121, 211, 0, 0, 0, 0, 0, 0, 0, 0, 172, 115, 111, 109, 101, 95, 97, 100, 100, 114, 101, 115, 115, 211, 0, 0, 0, 0, 0, 0, 0, 1, 160, 192, 192, 192}
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
			expected := []byte{92, 225, 92, 118, 18, 235, 66, 58, 221, 242, 171, 92, 28, 48, 141, 233, 105, 58, 36, 199, 132, 142, 162, 13, 215, 17, 119, 19, 43, 24, 34, 56, 7, 95, 10, 174, 255, 255, 57, 169, 206, 41, 192, 116, 29, 221, 93, 215, 134, 2, 174, 139, 30, 103, 112, 4, 252, 113, 39, 163, 179, 106, 207, 8}
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
			expected := util.BytesToHash([]byte{197, 179, 61, 197, 203, 21, 80, 116, 198, 29, 184, 18, 9, 188, 210, 73, 16, 194, 3, 95, 174, 7, 119, 5, 187, 115, 142, 48, 5, 153, 235, 83})
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
			Expect(tx.GetID()).To(Equal("0xc5b33dc5cb155074c61db81209bcd24910c2035fae077705bb738e300599eb53"))
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

	Describe("EpochSecretFromBytes", func() {

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

	Describe("UnbondTicket.Bytes", func() {
		It("should return expected bytes", func() {
			es := &UnbondTicket{TicketID: []byte("ticket_id")}
			bz := es.Bytes()
			Expect(bz).To(Equal([]byte{145, 196, 9, 116, 105, 99, 107, 101, 116, 95, 105, 100}))
		})
	})

	Describe("UnbondTicketFromBytes", func() {
		Context("with empty argument", func() {
			It("should return empty EpochSecret", func() {
				ut := UnbondTicketFromBytes([]byte{})
				Expect(ut).To(Equal(&UnbondTicket{}))
			})
		})

		Context("with invalid/malformed bytes", func() {
			It("should panic", func() {
				Expect(func() {
					UnbondTicketFromBytes([]byte{1, 2, 3})
				}).To(Panic())
			})
		})

		Context("with valid bytes", func() {
			bz := []byte{145, 196, 9, 116, 105, 99, 107, 101, 116, 95, 105, 100}
			ut := &UnbondTicket{TicketID: []byte("ticket_id")}
			It("should return expected object", func() {
				res := UnbondTicketFromBytes(bz)
				Expect(res).To(Equal(ut))
			})
		})
	})

	Describe("RepoCreate.Bytes", func() {
		It("should return expected bytes", func() {
			rc := &RepoCreate{Name: "my_repo"}
			bz := rc.Bytes()
			Expect(bz).To(Equal([]byte{0x91, 0xa7, 0x6d, 0x79, 0x5f, 0x72, 0x65, 0x70, 0x6f}))
		})
	})

	Describe("RepoCreateFromBytes", func() {
		Context("with empty argument", func() {
			It("should return empty EpochSecret", func() {
				ut := RepoCreateFromBytes([]byte{})
				Expect(ut).To(Equal(&RepoCreate{}))
			})
		})

		Context("with invalid/malformed bytes", func() {
			It("should panic", func() {
				Expect(func() {
					RepoCreateFromBytes([]byte{1, 2, 3})
				}).To(Panic())
			})
		})

		Context("with valid bytes", func() {
			bz := []byte{0x91, 0xa7, 0x6d, 0x79, 0x5f, 0x72, 0x65, 0x70, 0x6f}
			rc := &RepoCreate{Name: "my_repo"}
			It("should return expected object", func() {
				res := RepoCreateFromBytes(bz)
				Expect(res).To(Equal(rc))
			})
		})
	})
})
