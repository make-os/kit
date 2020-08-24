package pool

import (
	"time"

	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/txns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("pool", func() {

	Describe(".Put", func() {
		It("should return err = 'capacity reached' when pool capacity is reached", func() {
			tp := New(0)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := tp.Put(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrContainerFull))
		})

		It("should return err = 'exact transaction already in the pool' when transaction has already been added", func() {
			tp := New(10)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := tp.Put(tx)
			Expect(err).To(BeNil())
			err = tp.Put(tx)
			Expect(err).To(Equal(ErrTxAlreadyAdded))
		})

		It("should return nil and added to queue", func() {
			tp := New(1)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			err := tp.Put(tx)
			Expect(err).To(BeNil())
			Expect(tp.container.Size()).To(Equal(1))
		})
	})

	Describe(".Has", func() {

		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			tp = New(1)
		})

		It("should return true when tx exist", func() {
			tx := txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)
			Expect(tp.Has(tx)).To(BeTrue())
		})

		It("should return false when tx does not exist", func() {
			tx := txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			Expect(tp.Has(tx)).To(BeFalse())
		})
	})

	Describe(".Size", func() {

		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			tp = New(1)
			Expect(tp.Size()).To(Equal(0))
		})

		It("should return 1", func() {
			tx := txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)
			Expect(tp.Size()).To(Equal(1))
		})
	})

	Describe(".ByteSize", func() {

		var tx, tx2 types.BaseTx
		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)

		BeforeEach(func() {
			tp = New(2)
		})

		BeforeEach(func() {
			tx = txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			tx2 = txns.NewCoinTransferTx(100, "something_2", sender2, "0", "0", time.Now().Unix())
			tp.Put(tx)
			tp.Put(tx2)
		})

		It("should return expected byte size", func() {
			s := tp.ByteSize()
			Expect(s).To(Equal(tx.GetEcoSize() + tx2.GetEcoSize()))
		})

		When("a transaction is removed", func() {

			var curByteSize int64

			BeforeEach(func() {
				curByteSize = tp.ByteSize()
				Expect(curByteSize).To(Equal(tx.GetEcoSize() + tx2.GetEcoSize()))
			})

			It("should reduce the byte size when First is called", func() {
				rmTx := tp.container.First()
				s := tp.ByteSize()
				Expect(s).To(Equal(curByteSize - rmTx.GetEcoSize()))
			})

			It("should reduce the byte size when Last is called", func() {
				rmTx := tp.container.Last()
				s := tp.ByteSize()
				Expect(s).To(Equal(curByteSize - rmTx.GetEcoSize()))
			})
		})
	})

	Describe(".ActualSize", func() {

		var tx, tx2 types.BaseTx
		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)

		BeforeEach(func() {
			tp = New(2)
		})

		BeforeEach(func() {
			tx = txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			tx2 = txns.NewCoinTransferTx(100, "something_2", sender2, "0", "0", time.Now().Unix())
			tp.Put(tx)
			tp.Put(tx2)
		})

		It("should return expected actual size", func() {
			s := tp.ActualSize()
			Expect(s).To(Equal(tx.GetSize() + tx2.GetSize()))
		})

		When("a transaction is removed", func() {

			var curByteSize int64

			BeforeEach(func() {
				curByteSize = tp.ActualSize()
				Expect(curByteSize).To(Equal(tx.GetSize() + tx2.GetSize()))
			})

			It("should reduce the actual byte size when First is called", func() {
				rmTx := tp.container.First()
				s := tp.ActualSize()
				Expect(s).To(Equal(curByteSize - rmTx.GetSize()))
			})

			It("should reduce the actual byte size when Last is called", func() {
				rmTx := tp.container.Last()
				s := tp.ActualSize()
				Expect(s).To(Equal(curByteSize - rmTx.GetSize()))
			})
		})
	})

	Describe(".clean", func() {

		var tx, tx2 types.BaseTx
		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when TxTTL is 1 day", func() {

			BeforeEach(func() {
				params.TxTTL = 1
				tp = New(2)

				tx = txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
				tx.SetTimestamp(time.Now().UTC().AddDate(0, 0, -2).Unix())

				tx2 = txns.NewCoinTransferTx(101, "something2", sender, "0", "0", time.Now().Unix())
				tx2.SetTimestamp(time.Now().Unix())

				tp.container.Add(tx)
				tp.container.Add(tx2)
				Expect(tp.Size()).To(Equal(2))
			})

			It("should remove expired transaction", func() {
				tp.clean()
				Expect(tp.Size()).To(Equal(1))
				Expect(tp.Has(tx2)).To(BeTrue())
				Expect(tp.Has(tx)).To(BeFalse())
			})
		})
	})

	Describe(".Remove", func() {

		var tp *Pool
		var tx, tx2, tx3 types.BaseTx
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)
		var sender3 = crypto.NewKeyFromIntSeed(3)

		BeforeEach(func() {
			tp = New(100)

			tx = txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)

			tx2 = txns.NewCoinTransferTx(100, "something2", sender2, "0", "0", time.Now().Unix())
			tp.Put(tx2)

			tx3 = txns.NewCoinTransferTx(100, "something3", sender3, "0", "0", time.Now().Unix())
			tp.Put(tx3)
		})

		It("should remove the transactions included in the block", func() {
			txs := []types.BaseTx{tx2, tx3}
			tp.Remove(txs...)
			Expect(tp.Size()).To(Equal(1))
			Expect(tp.container.Get(0).Tx).To(Equal(tx))
		})
	})

	Describe(".GetByHash", func() {

		var tp *Pool
		var tx, tx2 types.BaseTx
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)

		BeforeEach(func() {
			tp = New(100)

			tx = txns.NewCoinTransferTx(100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)

			tx2 = txns.NewCoinTransferTx(100, "something2", sender2, "0", "0", time.Now().Unix())
		})

		It("It should not be equal", func() {
			Expect(tx).ToNot(Equal(tx2))
		})

		It("should get transaction from pool", func() {
			txData := tp.GetByHash(tx.GetHash().String())
			Expect(txData).ToNot(BeNil())
			Expect(txData).To(Equal(tx))
		})

		It("should return nil from  GetTransaction in pool", func() {
			txData := tp.GetByHash(tx2.GetHash().String())
			Expect(txData).To(BeNil())
		})

	})

})
