package pool

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util/identifier"
	"github.com/olebedev/emitter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pool Suite")
}

var _ = Describe("pool", func() {
	var ctrl *gomock.Controller
	var mockKeepers *mocks.MockKeepers
	var mockAcctKeeper *mocks.MockAccountKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockAcctKeeper = mocks.NewMockAccountKeeper(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".Put", func() {
		It("should return err = 'capacity reached' when pool capacity is reached", func() {
			tp := New(0, nil, nil)
			sender := crypto.NewKeyFromIntSeed(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			_, err := tp.Put(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrContainerFull))
		})

		It("should return err = 'exact transaction already in the pool' when transaction has already been added", func() {
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount())
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper)

			tp := New(10, mockKeepers, emitter.New(10))
			sender := crypto.NewKeyFromIntSeed(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			_, err := tp.Put(tx)
			Expect(err).To(BeNil())
			_, err = tp.Put(tx)
			Expect(err).To(Equal(ErrTxAlreadyAdded))
		})

		It("should return nil and added to queue", func() {
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount())
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper)

			tp := New(1, mockKeepers, emitter.New(10))
			sender := crypto.NewKeyFromIntSeed(1)
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			_, err := tp.Put(tx)
			Expect(err).To(BeNil())
			Expect(tp.container.Size()).To(Equal(1))
		})
	})

	Describe(".getNonce", func() {
		It("should return ErrAccountUnknown if account does not exist", func() {
			mockAcctKeeper.EXPECT().Get(identifier.Address("some_address")).Return(state.BareAccount())
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper)
			tp := New(1, mockKeepers, emitter.New(10))
			_, err := tp.getNonce("some_address")
			Expect(err).To(Equal(types.ErrAccountUnknown))
		})

		It("should return nonce if account exist", func() {
			acct := state.BareAccount()
			acct.Nonce = 10
			mockAcctKeeper.EXPECT().Get(identifier.Address("some_address")).Return(acct)
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper)
			tp := New(1, mockKeepers, emitter.New(10))
			nonce, err := tp.getNonce("some_address")
			Expect(err).To(BeNil())
			Expect(nonce).To(Equal(uint64(10)))
		})
	})

	Describe(".Has", func() {

		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount()).AnyTimes()
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
			tp = New(1, mockKeepers, emitter.New(10))
		})

		It("should return true when tx exist", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)
			Expect(tp.Has(tx)).To(BeTrue())
		})

		It("should return false when tx does not exist", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			Expect(tp.Has(tx)).To(BeFalse())
		})
	})

	Describe(".Size", func() {

		var tp *Pool
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount()).AnyTimes()
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()

			tp = New(1, mockKeepers, emitter.New(10))
			Expect(tp.Size()).To(Equal(0))
		})

		It("should return 1", func() {
			tx := txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
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
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount()).AnyTimes()
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
			tp = New(2, mockKeepers, emitter.New(10))
		})

		BeforeEach(func() {
			tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			tx2 = txns.NewCoinTransferTx(1, "something_2", sender2, "0", "0", time.Now().Unix())
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

	Describe(".Remove", func() {

		var tp *Pool
		var tx, tx2, tx3 types.BaseTx
		var sender = crypto.NewKeyFromIntSeed(1)
		var sender2 = crypto.NewKeyFromIntSeed(2)
		var sender3 = crypto.NewKeyFromIntSeed(3)

		BeforeEach(func() {
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount()).AnyTimes()
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
			tp = New(100, mockKeepers, emitter.New(10))

			tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)

			tx2 = txns.NewCoinTransferTx(1, "something2", sender2, "0", "0", time.Now().Unix())
			tp.Put(tx2)

			tx3 = txns.NewCoinTransferTx(1, "something3", sender3, "0", "0", time.Now().Unix())
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
			mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.BareAccount()).AnyTimes()
			mockKeepers.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
			tp = New(100, mockKeepers, emitter.New(10))

			tx = txns.NewCoinTransferTx(1, "something", sender, "0", "0", time.Now().Unix())
			tp.Put(tx)

			tx2 = txns.NewCoinTransferTx(1, "something2", sender2, "0", "0", time.Now().Unix())
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
