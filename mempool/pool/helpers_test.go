package pool

import (
	"time"

	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("senderNonces", func() {
	var sender = crypto.NewKeyFromIntSeed(1)
	var tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
	var tx2 = txns.NewCoinTransferTx(2, "something", sender, "0", "0.2", time.Now().Unix())
	var nc *nonceCollection
	var sn senderNonces

	BeforeEach(func() {
		nc = newNonceCollection()
		sn = map[identifier.Address]*nonceCollection{}
	})

	Describe(".remove", func() {
		When("sender address is not in the collection", func() {
			BeforeEach(func() {
				sn["addr_1"] = newNonceCollection()
				sn["addr_2"] = nc
				nc.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash()})
				nc.add(tx2.GetNonce(), &nonceInfo{TxHash: tx2.GetHash()})
				sn.remove("addr_3", tx.GetNonce())
			})

			It("should leave the collection unchanged", func() {
				Expect(nc.nonces).To(HaveLen(2))
			})
		})

		When("sender address is in the collection and has one transaction", func() {
			BeforeEach(func() {
				sn[tx.GetFrom()] = nc
				Expect(sn.len()).To(Equal(1))
				nc.add(tx.GetNonce(), &nonceInfo{TxHash: tx.GetHash()})
				Expect(nc.nonces).To(HaveLen(1))
				sn.remove(tx.GetFrom(), tx.GetNonce())
			})

			Specify("that nonce has been deleted", func() {
				Expect(nc.nonces).To(HaveLen(0))
			})

			Specify("that the sender address record is removed since it has no nonce", func() {
				Expect(sn.len()).To(Equal(0))
			})
		})
	})
})

var _ = Describe("NonceCollection", func() {
	nc := newNonceCollection()

	Describe(".has", func() {
		Context("when nonce is not part of the collection", func() {
			It("should return false", func() {
				Expect(nc.has(1)).To(BeFalse())
			})
		})

		Context("when nonce is part of the collection", func() {
			nc := nonceCollection{
				nonces: map[uint64]*nonceInfo{
					1: {TxHash: util.HexBytes{1, 2}},
				},
			}

			It("should return false", func() {
				Expect(nc.has(1)).To(BeTrue())
			})
		})
	})

	Describe(".Add", func() {
		BeforeEach(func() {
			nc.add(1, &nonceInfo{})
			Expect(nc.nonces).To(HaveLen(1))
		})

		It("should Add nonce", func() {
			Expect(nc.has(1)).To(BeTrue())
		})
	})

	Describe(".get", func() {
		nonce := &nonceInfo{TxHash: util.HexBytes{1, 2}}
		BeforeEach(func() {
			nc.add(1, nonce)
			Expect(nc.nonces).To(HaveLen(1))
		})

		It("should get nonce", func() {
			res := nc.get(1)
			Expect(res).To(Equal(nonce))
		})

		When("nonce does not exist", func() {
			It("should return nil", func() {
				res := nc.get(2)
				Expect(res).To(BeNil())
			})
		})
	})

	Describe(".remove", func() {
		BeforeEach(func() {
			nc.add(1, &nonceInfo{})
			Expect(nc.nonces).To(HaveLen(1))
		})

		It("should Add nonce", func() {
			nc.remove(1)
			Expect(nc.has(1)).To(BeFalse())
			Expect(nc.nonces).To(HaveLen(0))
		})
	})
})

var _ = Describe("Cache", func() {
	var c *Cache
	var sender = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		c = NewCache()
	})

	Describe(".Add", func() {
		It("should add tx successfully", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())
		})

		It("should return false if tx has already been added", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())
			err = c.Add(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("cache already contains a transaction with matching sender and nonce"))
		})

		It("should return false if tx has been seen before and is considered expired", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())
			Expect(c.firstSeen.Has(tx.GetID())).To(BeTrue())

			// Get/remove the tx from the cache
			c.Get()

			// Set Mempool TTL and sleep for some time so time passes
			params.MempoolTxTTL = 3 * time.Millisecond
			time.Sleep(5 * time.Millisecond)

			// Add it again
			err = c.Add(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("refused to cache old transaction"))

			// Should remove the tx from the first seen list
			Expect(c.firstSeen.Has(tx.GetID())).To(BeFalse())
		})
	})

	Describe(".markFirstSeenTime", func() {
		It("should add tx to firstSeen cache", func() {
			c.markFirstSeenTime("hash")
			Expect(c.firstSeen.Has("hash")).To(BeTrue())
		})

		It("should not replace value of existing entry with matching hash", func() {
			c.markFirstSeenTime("hash")
			val := c.firstSeen.Get("hash")
			c.markFirstSeenTime("hash")
			val2 := c.firstSeen.Get("hash")
			Expect(val).To(Equal(val2))
		})
	})

	Describe(".getFirstSeen", func() {
		It("should add tx to firstSeen cache", func() {
			c.markFirstSeenTime("hash")
			Expect(c.firstSeen.Has("hash")).To(BeTrue())
			t := c.getFirstSeen("hash")
			Expect(t).ToNot(BeNil())
		})
	})

	Describe(".SizeByAddr", func() {
		It("should return 1 if sender has a single tx", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())
			Expect(c.SizeByAddr(sender.Addr())).To(Equal(1))
		})

		It("should return 2 if sender has a two txs", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())

			tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "0", time.Now().Unix())
			err = c.Add(tx2)
			Expect(err).To(BeNil())

			Expect(c.SizeByAddr(sender.Addr())).To(Equal(2))
		})
	})

	Describe(".Size", func() {
		It("should return cache size", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())
			Expect(c.Size()).To(Equal(1))
		})
	})

	Describe(".Get", func() {
		It("should return nil if no tx in the cache", func() {
			tx := c.Get()
			Expect(tx).To(BeNil())
		})

		It("should return tx at the head of the cache (channel)", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := c.Add(tx)
			Expect(err).To(BeNil())

			tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "0", time.Now().Unix())
			err = c.Add(tx2)
			Expect(err).To(BeNil())

			Expect(c.Get()).To(Equal(tx))
			Expect(c.Get()).To(Equal(tx2))
		})
	})

	Describe(".Has", func() {
		It("should return nil if no tx in the cache", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			has := c.Has(tx)
			Expect(has).To(BeFalse())

			err := c.Add(tx)
			Expect(err).To(BeNil())
			has = c.Has(tx)
			Expect(has).To(BeTrue())
		})
	})
})
