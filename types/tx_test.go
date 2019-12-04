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
			// Expect(func() {
			NewTx(TxTypeCoinTransfer, 0, "recipient_addr", address, "10", "0.1", time.Now().Unix())
			// }).ToNot(Panic())
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
			Expect(tx.ToMap()).ToNot(BeEmpty())
		})
	})

	Describe(".Bytes", func() {

		It("should return expected bytes", func() {
			tx := &Transaction{Type: 1, Nonce: 1, To: "some_address", SenderPubKey: "some_pub_key"}
			bs := tx.GetBytesNoSig()
			Expect(bs).ToNot(BeEmpty())
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
			Expect(err).To(BeNil())
			Expect(sig).ToNot(BeEmpty())
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
			hash := tx.ComputeHash()
			Expect(hash).To(HaveLen(32))
		})
	})

	Describe(".GetID", func() {
		It("should return expected transaction ID", func() {
			sender := crypto.NewKeyFromIntSeed(1)
			tx := NewBareTx(1)
			tx.Timestamp = 1
			tx.SetSenderPubKey(util.String(sender.PubKey().Base58()))
			tx.To = "address_1"
			Expect(len(tx.GetID())).To(Equal(66))
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

	Describe("AddGPGPubKey.Bytes", func() {
		It("should return expected bytes", func() {
			a := &AddGPGPubKey{PublicKey: "---PUBKEY..."}
			bz := a.Bytes()
			Expect(bz).ToNot(BeEmpty())
		})
	})

	Describe("AddGPGPubKeyFromBytes", func() {
		Context("with empty argument", func() {
			It("should return empty AddGPGPubKey", func() {
				ut := AddGPGPubKeyFromBytes([]byte{})
				Expect(ut).To(Equal(&AddGPGPubKey{}))
			})
		})

		Context("with invalid/malformed bytes", func() {
			It("should panic", func() {
				Expect(func() {
					AddGPGPubKeyFromBytes([]byte{1, 2, 3})
				}).To(Panic())
			})
		})

		Context("with valid bytes", func() {
			bz := []uint8{0x91, 0xac, 0x2d, 0x2d, 0x2d, 0x50, 0x55, 0x42, 0x4b, 0x45, 0x59, 0x2e, 0x2e, 0x2e}
			a := &AddGPGPubKey{PublicKey: "---PUBKEY..."}
			It("should return expected object", func() {
				res := AddGPGPubKeyFromBytes(bz)
				Expect(res).To(Equal(a))
			})
		})
	})
})
