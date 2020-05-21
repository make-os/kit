package pool

import (
	"time"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxContainer", func() {

	var sender = crypto.NewKeyFromIntSeed(1)
	var sender2 = crypto.NewKeyFromIntSeed(2)

	Describe(".Register", func() {
		It("should return ErrContainerFull when capacity is reached", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			q := newTxContainer(0)
			Expect(q.add(tx)).To(Equal(ErrContainerFull))
		})

		It("should return nil when transaction is successfully added", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			q := newTxContainer(1)
			Expect(q.add(tx)).To(BeNil())
			Expect(q.container).To(HaveLen(1))
		})

		When("sorting is disabled", func() {
			It("should return transactions in the following order tx2, tx1", func() {
				tx1 := txns.NewCoinTransferTx(1, "something", sender, "0", "0.10", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "something", sender2, "0", "1", time.Now().Unix())
				q := NewQueueNoSort(2)
				q.add(tx1)
				q.add(tx2)
				Expect(q.Size()).To(Equal(int64(2)))
				Expect(q.container[0].Tx).To(Equal(tx1))
				Expect(q.container[1].Tx).To(Equal(tx2))
			})
		})

		When("sender has two transactions with same nonce and same fee rate", func() {
			Specify("that error is returned when attempting to add the second transaction", func() {
				q := newTxContainer(2)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				err := q.add(tx)
				Expect(err).To(BeNil())
				Expect(q.container).To(HaveLen(1))

				err = q.add(tx2)
				Expect(err).To(Equal(ErrFailedReplaceByFee))
			})
		})
	})

	Describe(".Size", func() {
		It("should return size = 1", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			q := newTxContainer(2)
			Expect(q.add(tx)).To(BeNil())
			Expect(q.Size()).To(Equal(int64(1)))
		})
	})

	Describe(".First", func() {

		It("should return nil when queue is empty", func() {
			q := newTxContainer(2)
			Expect(q.First()).To(BeNil())
		})

		Context("with sorting disabled", func() {
			It("should return first transaction in the queue and reduce queue size to 1", func() {
				q := NewQueueNoSort(2)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "0", time.Now().Unix())
				q.add(tx)
				q.add(tx2)
				Expect(q.First()).To(Equal(tx))
				Expect(q.Size()).To(Equal(int64(1)))
				Expect(q.container[0].Tx).To(Equal(tx2))
				Expect(q.Size()).To(Equal(int64(1)))
			})
		})

		Context("with sorting enabled", func() {

			When("sender has two transactions with same nonce and different fee rate", func() {
				Specify("that only one transaction exist in the pool and the transaction has the higher fee rate", func() {
					q := newTxContainer(2)
					tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
					tx2 := txns.NewCoinTransferTx(1, "something", sender, "0", "1.2", time.Now().Unix())
					err := q.add(tx)
					Expect(err).To(BeNil())
					Expect(q.container).To(HaveLen(1))
					err = q.add(tx2)
					Expect(q.container).To(HaveLen(1))
					Expect(err).To(BeNil())
					Expect(q.First()).To(Equal(tx2))
				})
			})

			When("sender has two transaction with different nonce", func() {
				It("after sorting, the first transaction must be the one with the lowest nonce", func() {
					q := newTxContainer(2)
					tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
					tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
					q.add(tx)
					q.add(tx2)
					Expect(q.container).To(HaveLen(2))
					Expect(q.First()).To(Equal(tx))
					Expect(q.Size()).To(Equal(int64(1)))
				})
			})

			When("container has 2 transactions from a sender and one from a different sender", func() {
				It("after sorting, the first transaction must be the one with the highest fee rate", func() {
					sender2 := crypto.NewKeyFromIntSeed(2)
					q := newTxContainer(3)
					tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
					tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
					tx3 := txns.NewCoinTransferTx(2, "something", sender2, "0", "2", time.Now().Unix())
					q.add(tx)
					q.add(tx2)
					q.add(tx3)
					Expect(q.container).To(HaveLen(3))
					Expect(q.First()).To(Equal(tx3))
					Expect(q.Size()).To(Equal(int64(2)))
					Expect(q.container[0].Tx).To(Equal(tx))
					Expect(q.container[1].Tx).To(Equal(tx2))
				})
			})
		})
	})

	Describe(".Last", func() {
		It("should return nil when queue is empty", func() {
			q := newTxContainer(2)
			Expect(q.Last()).To(BeNil())
		})

		Context("with sorting disabled", func() {
			It("should return last transaction in the queue and reduce queue size to 1", func() {
				q := newTxContainer(2)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "0", time.Now().Unix())
				q.add(tx)
				q.add(tx2)
				Expect(q.Last()).To(Equal(tx2))
				Expect(q.Size()).To(Equal(int64(1)))
			})
		})

		When("sender has two transaction with different nonce", func() {
			It("after sorting, the last transaction must be the one with the highest nonce", func() {
				q := newTxContainer(2)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
				q.add(tx)
				q.add(tx2)
				Expect(q.container).To(HaveLen(2))
				Expect(q.Last()).To(Equal(tx2))
				Expect(q.Size()).To(Equal(int64(1)))
			})
		})

		When("container has 2 transactions from a sender (A) and one from a different sender (B)", func() {
			It("after sorting, the last transaction must be sender (A) transaction with the highest nonce", func() {
				sender2 := crypto.NewKeyFromIntSeed(2)
				q := newTxContainer(3)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
				tx3 := txns.NewCoinTransferTx(2, "something", sender2, "0", "2", time.Now().Unix())
				q.add(tx)
				q.add(tx2)
				q.add(tx3)
				Expect(q.container).To(HaveLen(3))
				Expect(q.Last()).To(Equal(tx2))
				Expect(q.Size()).To(Equal(int64(2)))
				Expect(q.container[0].Tx).To(Equal(tx3))
				Expect(q.container[1].Tx).To(Equal(tx))
			})
		})
	})

	Describe(".Sort", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)

		It("with 2 transactions by same sender; sort by nonce in ascending order", func() {
			q := newTxContainer(2)
			items := []*containerItem{
				{Tx: txns.NewCoinTransferTx(2, "", sender, "10", "0", 0)},
				{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0)},
			}
			q.container = append(q.container, items...)
			q.Sort()
			Expect(q.container[0]).To(Equal(items[1]))
		})

		It("with 2 transactions by same sender; same nonce; sort by fee rate in descending order", func() {
			q := newTxContainer(2)
			items := []*containerItem{
				{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0), FeeRate: "0.1"},
				{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0), FeeRate: "0.2"},
			}
			q.container = append(q.container, items...)
			q.Sort()
			Expect(q.container[0]).To(Equal(items[1]))
		})

		Specify(`3 transactions; 
				2 by same sender and different nonce; 
				1 with highest fee rate; 
				sort by nonce (ascending) for the same sender txs;
				sort by fee rate (descending) for others`, func() {
			q := newTxContainer(2)
			items := []*containerItem{
				{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0), FeeRate: "0.1"},
				{Tx: txns.NewCoinTransferTx(2, "", sender, "10", "0", 0), FeeRate: "0.2"},
				{Tx: txns.NewCoinTransferTx(4, "", sender2, "10", "0", 0), FeeRate: "1.2"},
			}
			q.container = append(q.container, items...)
			q.Sort()
			Expect(q.container[0]).To(Equal(items[2]))
			Expect(q.container[1]).To(Equal(items[0]))
			Expect(q.container[2]).To(Equal(items[1]))
		})
	})

	Describe(".Has", func() {
		It("should return true when tx exist in queue", func() {
			q := newTxContainer(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			err := q.add(tx)
			Expect(err).To(BeNil())
			has := q.Has(tx)
			Expect(has).To(BeTrue())
		})

		It("should return false when tx does not exist in queue", func() {
			q := newTxContainer(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			has := q.Has(tx)
			Expect(has).To(BeFalse())
		})
	})

	Describe(".HasByHash", func() {
		It("should return true when tx exist in queue", func() {
			q := newTxContainer(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			err := q.add(tx)
			Expect(err).To(BeNil())
			has := q.HasByHash(tx.GetHash().HexStr())
			Expect(has).To(BeTrue())
		})

		It("should return false when tx does not exist in queue", func() {
			q := newTxContainer(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			has := q.HasByHash(tx.GetHash().HexStr())
			Expect(has).To(BeFalse())
		})
	})

	Describe(".remove", func() {

		var q *TxContainer
		var tx, tx2, tx3, tx4 types.BaseTx

		BeforeEach(func() {
			q = newTxContainer(4)
			tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			q.add(tx)
			tx2 = txns.NewCoinTransferTx(2, "something2", sender, "0", "0.2", time.Now().Unix())
			q.add(tx2)
			tx3 = txns.NewCoinTransferTx(3, "something2", sender, "0", "0.2", time.Now().Unix())
			q.add(tx3)
			tx4 = txns.NewCoinTransferTx(4, "something2", sender, "0", "0.4", time.Now().Unix())
			q.add(tx4)
			Expect(q.Size()).To(Equal(int64(4)))
		})

		It("should do nothing when transaction does not exist in the container", func() {
			unknownTx := txns.NewCoinTransferTx(1, "unknown", sender, "0", "0.2", time.Now().Unix())
			q.Remove(unknownTx)
			Expect(q.Size()).To(Equal(int64(4)))
		})

		It("should remove transactions", func() {
			q.Remove(tx2, tx3)
			Expect(q.Size()).To(Equal(int64(2)))
			Expect(q.container[0].Tx).To(Equal(tx))
			Expect(q.container[1].Tx).To(Equal(tx4))
			Expect(q.len).To(Equal(int64(2)))
			Expect(q.byteSize).To(Equal(tx.GetEcoSize() + tx4.GetEcoSize()))
		})
	})

	Describe(".Get", func() {

		var q *TxContainer
		var tx1, tx2, tx3 types.BaseTx

		BeforeEach(func() {
			q = newTxContainer(3)
			tx1 = txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			tx2 = txns.NewCoinTransferTx(2, "something", sender, "0", "0.2", time.Now().Unix())
			tx3 = txns.NewCoinTransferTx(3, "something", sender, "0", "0.2", time.Now().Unix())
			q.add(tx1)
			q.add(tx2)
			q.add(tx3)
		})

		It("should stop iterating when predicate returns true", func() {
			var iterated []types.BaseTx
			result := q.Find(func(tx types.BaseTx) bool {
				iterated = append(iterated, tx)
				return tx.GetNonce() == 2
			})

			Describe("it should return the last item sent to the predicate", func() {
				Expect(result).To(Equal(tx2))
			})

			Describe("it should contain the first and second transaction and not the 3rd transaction", func() {
				Expect(iterated).To(HaveLen(2))
				Expect(iterated).ToNot(ContainElement(tx3))
			})
		})

		It("should return nil when predicate did not return true", func() {
			var iterated []types.BaseTx
			result := q.Find(func(tx types.BaseTx) bool {
				iterated = append(iterated, tx)
				return false
			})
			Expect(result).To(BeNil())

			Describe("it should contain all transactions", func() {
				Expect(iterated).To(HaveLen(3))
			})
		})
	})

	Describe(".Get", func() {
		It("should return Not nil when tx exist in queue", func() {
			q := newTxContainer(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			err := q.add(tx)
			Expect(err).To(BeNil())
			txData := q.GetByHash(tx.GetHash().HexStr())
			Expect(txData).ToNot(BeNil())
		})

		It("should return nil when tx does not exist in queue", func() {
			q := newTxContainer(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			txData := q.GetByHash(tx.GetHash().HexStr())
			Expect(txData).To(BeNil())
		})

	})

})

var _ = Describe("senderNonces", func() {
	var sender = crypto.NewKeyFromIntSeed(1)
	var tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
	var tx2 = txns.NewCoinTransferTx(2, "something", sender, "0", "0.2", time.Now().Unix())
	var nc *nonceCollection
	var sn senderNonces

	BeforeEach(func() {
		nc = defaultNonceCollection()
		sn = map[util.Address]*nonceCollection{}
	})

	Describe(".remove", func() {
		When("sender address is not in the collection", func() {
			BeforeEach(func() {
				sn["addr_1"] = defaultNonceCollection()
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
	nc := defaultNonceCollection()

	Describe(".has", func() {
		Context("when nonce is not part of the collection", func() {
			It("should return false", func() {
				Expect(nc.has(1)).To(BeFalse())
			})
		})

		Context("when nonce is part of the collection", func() {
			nc := nonceCollection{
				nonces: map[uint64]*nonceInfo{
					1: {TxHash: util.StrToBytes32("")},
				},
			}

			It("should return false", func() {
				Expect(nc.has(1)).To(BeTrue())
			})
		})
	})

	Describe(".add", func() {
		BeforeEach(func() {
			nc.add(1, &nonceInfo{})
			Expect(nc.nonces).To(HaveLen(1))
		})

		It("should add nonce", func() {
			Expect(nc.has(1)).To(BeTrue())
		})
	})

	Describe(".get", func() {
		nonce := &nonceInfo{TxHash: util.StrToBytes32("abc")}
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

		It("should add nonce", func() {
			nc.remove(1)
			Expect(nc.has(1)).To(BeFalse())
			Expect(nc.nonces).To(HaveLen(0))
		})
	})
})
