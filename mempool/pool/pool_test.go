package pool

import (
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/params"
)

var _ = Describe("pool", func() {

	Describe(".Put", func() {
		It("should return err = 'capacity reached' when pool capacity is reached", func() {
			tp := New(0)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := core.NewBaseTx(core.TxTypeCoinTransfer, 1, "something", sender, "0", "0", time.Now().Unix())
			err := tp.Put(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrContainerFull))
		})

		It("should return err = 'exact transaction already in the pool' when transaction has already been added", func() {
			tp := New(10)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := core.NewBaseTx(core.TxTypeCoinTransfer, 1, "something", sender, "0", "0", time.Now().Unix())
			err := tp.Put(tx)
			Expect(err).To(BeNil())
			err = tp.Put(tx)
			Expect(err).To(Equal(ErrTxAlreadyAdded))
		})

		It("should return nil and added to queue", func() {
			tp := New(1)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := core.NewBaseTx(core.TxTypeCoinTransfer, 1, "something", sender, "0", "0", time.Now().Unix())
			err := tp.Put(tx)
			Expect(err).To(BeNil())
			Expect(tp.container.Size()).To(Equal(int64(1)))
		})
	})

	Describe(".Has", func() {

		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			tp = New(1)
		})

		It("should return true when tx exist", func() {
			tx := core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)
			Expect(tp.Has(tx)).To(BeTrue())
		})

		It("should return false when tx does not exist", func() {
			tx := core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			Expect(tp.Has(tx)).To(BeFalse())
		})
	})

	Describe(".GetByFrom", func() {

		var tp *Pool
		var key1 = crypto.NewKeyFromIntSeed(1)
		var key2 = crypto.NewKeyFromIntSeed(2)
		var tx, tx2, tx3 types.BaseTx

		BeforeEach(func() {
			tp = New(3)
			tx = core.NewBaseTx(core.TxTypeCoinTransfer, 1, "a", key1, "12.2", "0.2", time.Now().Unix())
			tx2 = core.NewBaseTx(core.TxTypeCoinTransfer, 2, "a", key1, "12.3", "0.2", time.Now().Unix())
			tx3 = core.NewBaseTx(core.TxTypeCoinTransfer, 2, "a", key2, "12.3", "0.2", time.Now().Unix())
			_ = tp.addTx(tx)
			_ = tp.addTx(tx2)
			_ = tp.addTx(tx3)
			Expect(tp.Size()).To(Equal(int64(3)))
		})

		It("should return two transactions matching key1", func() {
			txs := tp.GetByFrom(key1.Addr())
			Expect(txs).To(HaveLen(2))
			Expect(txs[0]).To(Equal(tx))
			Expect(txs[1]).To(Equal(tx2))
		})
	})

	Describe(".Size", func() {

		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			tp = New(1)
			Expect(tp.Size()).To(Equal(int64(0)))
		})

		It("should return 1", func() {
			tx := core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)
			Expect(tp.Size()).To(Equal(int64(1)))
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
			tx = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			tx2 = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something_2", sender2, "0", "0", time.Now().Unix())
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
			tx = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			tx2 = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something_2", sender2, "0", "0", time.Now().Unix())
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

				tx = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
				tx.SetTimestamp(time.Now().UTC().AddDate(0, 0, -2).Unix())

				tx2 = core.NewBaseTx(core.TxTypeCoinTransfer, 101, "something2", sender, "0", "0", time.Now().Unix())
				tx2.SetTimestamp(time.Now().Unix())

				tp.container.add(tx)
				tp.container.add(tx2)
				Expect(tp.Size()).To(Equal(int64(2)))
			})

			It("should remove expired transaction", func() {
				tp.clean()
				Expect(tp.Size()).To(Equal(int64(1)))
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

			tx = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)

			tx2 = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something2", sender2, "0", "0", time.Now().Unix())
			tp.Put(tx2)

			tx3 = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something3", sender3, "0", "0", time.Now().Unix())
			tp.Put(tx3)
		})

		It("should remove the transactions included in the block", func() {
			txs := []types.BaseTx{tx2, tx3}
			tp.Remove(txs...)
			Expect(tp.Size()).To(Equal(int64(1)))
			Expect(tp.container.container[0].Tx).To(Equal(tx))
		})
	})

	Describe(".GetByHash", func() {

		var tp *Pool
		var tx, tx2 types.BaseTx
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)

		BeforeEach(func() {
			tp = New(100)

			tx = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)

			tx2 = core.NewBaseTx(core.TxTypeCoinTransfer, 100, "something2", sender2, "0", "0", time.Now().Unix())
		})

		It("It should not be equal", func() {
			Expect(tx).ToNot(Equal(tx2))
		})

		It("should get transaction from pool", func() {
			txData := tp.GetByHash(tx.GetHash().HexStr())
			Expect(txData).ToNot(BeNil())
			Expect(txData).To(Equal(tx))
		})

		It("should return nil from  GetTransaction in pool", func() {
			txData := tp.GetByHash(tx2.GetHash().HexStr())
			Expect(txData).To(BeNil())
		})

	})

})
