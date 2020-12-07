package pool

import (
	"time"

	"github.com/make-os/kit/crypto"
	types2 "github.com/make-os/kit/mempool/types"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/olebedev/emitter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func zeroNonceGetter(_ string) (uint64, error) {
	return 0, nil
}

var _ = Describe("Container", func() {

	var sender = crypto.NewKeyFromIntSeed(1)
	var sender2 = crypto.NewKeyFromIntSeed(2)

	Describe(".Add", func() {
		It("should return ErrContainerFull when capacity is reached", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			q := NewContainer(0, nil, nil)
			_, err := q.Add(tx)
			Expect(err).To(Equal(ErrContainerFull))
		})

		It("should return nil when transaction is successfully added", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			_, err := q.Add(tx)
			Expect(err).To(BeNil())
			Expect(q.Size()).To(Equal(1))
		})

		When("sorting is disabled", func() {
			It("should return transactions in the following order tx2, tx1", func() {
				tx1 := txns.NewCoinTransferTx(1, "something", sender, "0", "0.10", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "something", sender2, "0", "1", time.Now().Unix())
				q := NewTxContainerNoSort(2, emitter.New(10), zeroNonceGetter)
				q.Add(tx1)
				q.Add(tx2)
				Expect(q.Size()).To(Equal(2))
				Expect(q.Get(0).Tx).To(Equal(tx1))
				Expect(q.Get(1).Tx).To(Equal(tx2))
			})
		})

		When("sender has two transactions with same nonce and same fee rate", func() {
			Specify("that error is returned when attempting to Add the second transaction", func() {
				q := NewContainer(2, emitter.New(1), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(1))
				_, err = q.Add(tx2)
				Expect(err).To(Equal(ErrFailedReplaceByFee))
			})
		})

		When("sender has transaction in the pool and tries to add another with same nonce and higher fee", func() {
			It("should replace existing transaction", func() {
				q := NewContainer(2, emitter.New(1), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "something", sender, "0", "2", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(1))
				_, err = q.Add(tx2)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(1))
				Expect(q.Has(tx2)).To(BeTrue())
			})
		})

		When("pool capacity is 1 and sender has transaction in the pool and tries to add another with same nonce and higher fee", func() {
			It("should replace existing transaction", func() {
				q := NewContainer(1, emitter.New(1), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "something", sender, "0", "2", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(1))
				_, err = q.Add(tx2)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(1))
				Expect(q.Has(tx2)).To(BeTrue())
			})
		})

		When("tx has a lower nonce than the current account nonce", func() {
			It("should return error", func() {
				q := NewContainer(1, emitter.New(10), func(_ string) (uint64, error) {
					return 10, nil
				})
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("tx nonce cannot be less than or equal to current account nonce"))
			})
		})

		When("tx has a matching nonce as the current account nonce", func() {
			It("should return error", func() {
				q := NewContainer(1, emitter.New(10), func(_ string) (uint64, error) {
					return 10, nil
				})
				tx := txns.NewCoinTransferTx(10, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("tx nonce cannot be less than or equal to current account nonce"))
			})
		})

		When("tx nonce is not the expect next nonce and the nonce before the tx's nonce (n-1) is not in the pool", func() {
			It("should add tx to the cache", func() {
				q := NewContainer(1, emitter.New(10), func(_ string) (uint64, error) {
					return 10, nil
				})
				tx := txns.NewCoinTransferTx(12, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(0))
				Expect(q.cache.Size()).To(Equal(1))
				Expect(q.cache.Get()).To(Equal(tx))
			})
		})

		When("tx nonce is not the expect next nonce and the nonce before the tx's nonce (n-1) is in the pool", func() {
			It("should add tx to the pool", func() {
				q := NewContainer(2, emitter.New(10), func(_ string) (uint64, error) {
					return 10, nil
				})
				tx := txns.NewCoinTransferTx(11, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				tx2 := txns.NewCoinTransferTx(12, "something", sender, "0", "1", time.Now().Unix())
				_, err = q.Add(tx2)
				Expect(err).To(BeNil())
				Expect(q.Size()).To(Equal(2))
			})
		})

		When("tx with matching sender and nonce already exist in cache", func() {
			It("should return an error", func() {
				q := NewContainer(1, emitter.New(10), func(_ string) (uint64, error) { return 10, nil })
				tx := txns.NewCoinTransferTx(12, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				_, err = q.Add(tx)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("cache already contains a transaction with matching sender and nonce"))
			})
		})

		When("sender has exceeded the pool's per-sender tx limit", func() {
			It("should return an error", func() {
				params.MempoolSenderTxLimit = 1
				q := NewContainer(1, emitter.New(10), func(_ string) (uint64, error) { return 10, nil })
				tx := txns.NewCoinTransferTx(12, "something", sender, "0", "1", time.Now().Unix())
				_, err := q.Add(tx)
				Expect(err).To(BeNil())
				tx2 := txns.NewCoinTransferTx(13, "something", sender, "0", "1", time.Now().Unix())
				_, err = q.Add(tx2)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrSenderTxLimitReached))
			})
		})
	})

	Describe(".Size", func() {
		It("should return size = 1", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			q := NewContainer(2, emitter.New(1), zeroNonceGetter)
			_, err := q.Add(tx)
			Expect(err).To(BeNil())
			Expect(q.Size()).To(Equal(1))
		})
	})

	Describe(".SizeByAddr", func() {
		When("sender has tx in pool and cache", func() {
			It("should return 2", func() {
				q := NewContainer(2, emitter.New(10), func(_ string) (uint64, error) { return 1, nil })
				tx := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())

				_, err := q.Add(tx)
				Expect(err).To(BeNil())

				time.Sleep(1 * time.Millisecond) // wait for defer calls in q.Add to finish
				err = q.cache.Add(tx)
				Expect(err).To(BeNil())

				count := q.SizeByAddr(tx.GetFrom())
				Expect(count).To(Equal(2))
			})
		})

		When("sender has tx in pool only", func() {
			It("should return 1", func() {
				q := NewContainer(2, emitter.New(10), func(_ string) (uint64, error) { return 1, nil })
				tx := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())

				_, err := q.Add(tx)
				Expect(err).To(BeNil())

				count := q.SizeByAddr(tx.GetFrom())
				Expect(count).To(Equal(1))
			})
		})
	})

	Describe(".First", func() {

		It("should return nil when queue is empty", func() {
			q := NewContainer(2, emitter.New(1), zeroNonceGetter)
			Expect(q.First()).To(BeNil())
		})

		Context("with sorting disabled", func() {
			It("should return first transaction in the queue and reduce queue size to 1", func() {
				q := NewTxContainerNoSort(2, emitter.New(10), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "2", time.Now().Unix())
				q.Add(tx)
				q.Add(tx2)
				Expect(q.Size()).To(Equal(2))
				Expect(q.Get(0).Tx).To(Equal(tx))
				Expect(q.Get(1).Tx).To(Equal(tx2))
			})
		})

		Context("with sorting enabled", func() {
			When("sender has two transactions with same nonce and different fee rate", func() {
				Specify("that only one transaction exist in the pool and the transaction has the higher fee rate", func() {
					q := NewContainer(2, emitter.New(1), zeroNonceGetter)
					tx := txns.NewCoinTransferTx(1, "something", sender, "0", "1", time.Now().Unix())
					tx2 := txns.NewCoinTransferTx(1, "something", sender, "0", "1.2", time.Now().Unix())
					_, err := q.Add(tx)
					Expect(err).To(BeNil())
					Expect(q.Size()).To(Equal(1))
					_, err = q.Add(tx2)
					Expect(q.Size()).To(Equal(1))
					Expect(err).To(BeNil())
					Expect(q.First()).To(Equal(tx2))
				})
			})

			When("sender has two transaction with different nonce", func() {
				It("after sorting, the first transaction must be the one with the lowest nonce", func() {
					q := NewContainer(2, emitter.New(1), zeroNonceGetter)
					tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
					tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
					q.Add(tx)
					q.Add(tx2)
					Expect(q.Size()).To(Equal(2))
					Expect(q.First()).To(Equal(tx))
					Expect(q.Size()).To(Equal(1))
				})
			})

			When("container has 2 transactions from a sender and one from a different sender", func() {
				It("after sorting, the first transaction must be the one with the highest fee rate", func() {
					sender2 := crypto.NewKeyFromIntSeed(2)
					q := NewContainer(3, emitter.New(1), zeroNonceGetter)
					tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
					tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
					tx3 := txns.NewCoinTransferTx(1, "something", sender2, "0", "2", time.Now().Unix())
					q.Add(tx)
					q.Add(tx2)
					q.Add(tx3)
					Expect(q.Size()).To(Equal(3))
					Expect(q.Get(0).Tx).To(Equal(tx3))
					Expect(q.Get(1).Tx).To(Equal(tx))
					Expect(q.Get(2).Tx).To(Equal(tx2))
				})
			})
		})
	})

	Describe(".Last", func() {
		It("should return nil when queue is empty", func() {
			q := NewContainer(2, emitter.New(1), zeroNonceGetter)
			Expect(q.Last()).To(BeNil())
		})

		Context("with sorting disabled", func() {
			It("should return last transaction in the queue and reduce queue size to 1", func() {
				q := NewContainer(2, emitter.New(1), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "0", time.Now().Unix())
				q.Add(tx)
				q.Add(tx2)
				Expect(q.Last()).To(Equal(tx2))
				Expect(q.Size()).To(Equal(1))
			})
		})

		When("sender has two transaction with different nonce", func() {
			It("after sorting, the last transaction must be the one with the highest nonce", func() {
				q := NewContainer(2, emitter.New(1), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
				q.Add(tx)
				q.Add(tx2)
				Expect(q.Size()).To(Equal(2))
				Expect(q.Last()).To(Equal(tx2))
				Expect(q.Size()).To(Equal(1))
			})
		})

		When("container has 2 transactions from a sender (A) and one from a different sender (B)", func() {
			It("after sorting, the last transaction must be sender (A) transaction with the highest nonce", func() {
				sender2 := crypto.NewKeyFromIntSeed(2)
				q := NewContainer(3, emitter.New(1), zeroNonceGetter)
				tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "something", sender, "0", "1", time.Now().Unix())
				tx3 := txns.NewCoinTransferTx(1, "something", sender2, "0", "2", time.Now().Unix())
				q.Add(tx)
				q.Add(tx2)
				q.Add(tx3)
				Expect(q.Size()).To(Equal(3))
				Expect(q.Last()).To(Equal(tx2))
				Expect(q.Size()).To(Equal(2))
				Expect(q.Get(0).Tx).To(Equal(tx3))
				Expect(q.Get(1).Tx).To(Equal(tx))
			})
		})
	})

	Describe(".Sort", func() {
		var sender = crypto.NewKeyFromIntSeed(1)
		// var sender2 = crypto.NewKeyFromIntSeed(2)

		It("with 2 transactions by same sender; sort by nonce in ascending order", func() {
			q := NewContainer(2, emitter.New(1), zeroNonceGetter)
			items := []interface{}{
				&containerItem{Tx: txns.NewCoinTransferTx(2, "", sender, "10", "0", 0)},
				&containerItem{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0)},
			}
			q.container.Append(items...)
			q.Sort()
			Expect(q.Get(0)).To(Equal(items[1]))
		})

		It("with 2 transactions by same sender; same nonce; no fee rate sorting", func() {
			q := NewContainer(2, emitter.New(1), zeroNonceGetter)
			items := []interface{}{
				&containerItem{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0), FeeRate: "0.0001"},
				&containerItem{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0), FeeRate: "0.02"},
			}
			q.container.Append(items...)
			q.Sort()
			Expect(q.Get(0)).To(Equal(items[0]))
		})

		Specify(`3 transactions;
				2 by same sender and different nonce;
				1 with highest fee rate;
				sort by nonce (ascending) for the same sender txs;
				sort by fee rate (descending) for others`, func() {
			q := NewContainer(2, emitter.New(1), zeroNonceGetter)
			items := []interface{}{
				&containerItem{Tx: txns.NewCoinTransferTx(1, "", sender, "10", "0", 0), FeeRate: "0.1"},
				&containerItem{Tx: txns.NewCoinTransferTx(2, "", sender, "10", "0", 0), FeeRate: "0.2"},
				&containerItem{Tx: txns.NewCoinTransferTx(4, "", sender2, "10", "0", 0), FeeRate: "1.2"},
			}
			q.container.Append(items...)
			q.Sort()
			Expect(q.Get(0)).To(Equal(items[2]))
			Expect(q.Get(1)).To(Equal(items[0]))
			Expect(q.Get(2)).To(Equal(items[1]))
		})
	})

	Describe(".Has", func() {
		It("should return true when tx exist in queue", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			_, err := q.Add(tx)
			Expect(err).To(BeNil())
			has := q.Has(tx)
			Expect(has).To(BeTrue())
		})

		It("should return false when tx does not exist in queue", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			has := q.Has(tx)
			Expect(has).To(BeFalse())
		})
	})

	Describe(".HasByHash", func() {
		It("should return true when tx exist in queue", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			_, err := q.Add(tx)
			Expect(err).To(BeNil())
			has := q.HasByHash(tx.GetHash().String())
			Expect(has).To(BeTrue())
		})

		It("should return false when tx does not exist in queue", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			has := q.HasByHash(tx.GetHash().String())
			Expect(has).To(BeFalse())
		})
	})

	Describe(".remove", func() {

		var c *Container
		var tx, tx2, tx3, tx4 types.BaseTx

		BeforeEach(func() {
			c = NewContainer(4, emitter.New(1), zeroNonceGetter)
			tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			c.Add(tx)
			tx2 = txns.NewCoinTransferTx(2, "something2", sender, "0", "0.2", time.Now().Unix())
			c.Add(tx2)
			tx3 = txns.NewCoinTransferTx(3, "something2", sender, "0", "0.2", time.Now().Unix())
			c.Add(tx3)
			tx4 = txns.NewCoinTransferTx(4, "something2", sender, "0", "0.4", time.Now().Unix())
			c.Add(tx4)
			Expect(c.Size()).To(Equal(4))
		})

		It("should do nothing when transaction does not exist in the container", func() {
			unknownTx := txns.NewCoinTransferTx(1, "unknown", sender, "0", "0.2", time.Now().Unix())
			c.Remove(unknownTx)
			Expect(c.Size()).To(Equal(4))
		})

		It("should remove transactions", func() {
			c.Remove(tx2, tx3)
			Expect(c.Size()).To(Equal(2))
			Expect(c.Get(0).Tx).To(Equal(tx))
			Expect(c.Get(1).Tx).To(Equal(tx4))
			Expect(c.Size()).To(Equal(2))
			Expect(c.byteSize).To(Equal(tx.GetEcoSize() + tx4.GetEcoSize()))
		})
	})

	Describe(".maybeProcessCache", func() {
		var c *Container

		BeforeEach(func() {
			c = NewContainer(4, emitter.New(1), zeroNonceGetter)
		})

		It("should return false and nil error if cache is empty", func() {
			added, err := c.maybeProcessCache()
			Expect(added).To(BeFalse())
			Expect(err).To(BeNil())
		})

		It("should emit EvtMempoolBroadcastTx event when tx is added from cache", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			c.cache.Add(tx)
			Expect(c.cache.Size()).To(Equal(1))
			go c.maybeProcessCache()
			evt := <-c.bus.On(types2.EvtMempoolBroadcastTx)
			Expect(tx).To(Equal(evt.Args[0]))
		})

		It("should emit EvtMempoolTxRejected event when tx from cache failed to be added to the pool", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			c.Add(tx)
			Expect(c.Size()).To(Equal(1))
			time.Sleep(1 * time.Millisecond)

			// Add same tx to cache to force a duplicate error when attempting to add to pool container again
			c.cache.Add(tx)
			Expect(c.cache.Size()).To(Equal(1))
			time.Sleep(1 * time.Millisecond)

			go c.maybeProcessCache()
			evt := <-c.bus.On(types2.EvtMempoolTxRejected)
			Expect(evt.Args[0]).ToNot(BeNil())
			Expect(evt.Args[0]).To(MatchError(ErrFailedReplaceByFee))
			Expect(tx).To(Equal(evt.Args[1]))
		})
	})

	Describe(".Get", func() {

		var c *Container
		var tx1, tx2, tx3 types.BaseTx

		BeforeEach(func() {
			c = NewContainer(3, emitter.New(1), zeroNonceGetter)
			tx1 = txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			tx2 = txns.NewCoinTransferTx(2, "something", sender, "0", "0.2", time.Now().Unix())
			tx3 = txns.NewCoinTransferTx(3, "something", sender, "0", "0.2", time.Now().Unix())
			c.Add(tx1)
			c.Add(tx2)
			c.Add(tx3)
		})

		It("should stop iterating when predicate returns true", func() {
			var iterated []types.BaseTx
			result := c.find(func(tx types.BaseTx, _ util.String, _ time.Time) bool {
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
			result := c.find(func(tx types.BaseTx, _ util.String, _ time.Time) bool {
				iterated = append(iterated, tx)
				return false
			})
			Expect(result).To(BeNil())

			Describe("it should contain all transactions", func() {
				Expect(iterated).To(HaveLen(3))
			})
		})
	})

	Describe(".GetByHash", func() {
		It("should return Not nil when tx exist in queue", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			_, err := q.Add(tx)
			Expect(err).To(BeNil())
			txData := q.GetByHash(tx.GetHash().String())
			Expect(txData).ToNot(BeNil())
		})

		It("should return nil when tx does not exist in queue", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			txData := q.GetByHash(tx.GetHash().String())
			Expect(txData).To(BeNil())
		})
	})

	Describe(".GetFeeRateByHash", func() {
		It("should return non-empty result when transaction exist in the container", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			_, err := q.Add(tx)
			Expect(err).To(BeNil())
			feeRate := q.GetFeeRateByHash(tx.GetHash().String())
			Expect(feeRate).To(Equal(calcFeeRate(tx)))
		})

		It("should return empty result when transaction does not exist in the container", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			feeRate := q.GetFeeRateByHash(tx.GetHash().String())
			Expect(feeRate).To(BeEmpty())
		})
	})

	Describe(".Flush", func() {
		It("should clear container, caches and counters", func() {
			q := NewContainer(1, emitter.New(1), zeroNonceGetter)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0.2", time.Now().Unix())
			q.Add(tx)
			Expect(q.Size()).To(Equal(1))
			q.Flush()
			Expect(q.Size()).To(BeZero())
			Expect(q.byteSize).To(BeZero())
			Expect(q.hashIndex).To(BeEmpty())
			Expect(q.senderNonceIndex).To(BeEmpty())
		})
	})
})
