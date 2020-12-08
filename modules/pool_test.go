package modules_test

import (
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
)

var _ = Describe("PoolModule", func() {
	var m *modules.PoolModule
	var ctrl *gomock.Controller
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockPushPool *mocks.MockPushPool

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockPushPool = mocks.NewMockPushPool(ctrl)
		m = modules.NewPoolModule(mockMempoolReactor, mockPushPool)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespacePool)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".GetSize", func() {
		It("should return mempool size info only", func() {
			mockMempoolReactor.EXPECT().GetPoolSize().Return(&core.PoolSizeInfo{TotalTxSize: 100, TxCount: 3})
			res := m.GetSize()
			Expect(res).To(HaveKey("size"))
			Expect(res["size"]).To(Equal(int64(100)))
			Expect(res).To(HaveKey("count"))
			Expect(res["count"]).To(Equal(3))
		})
	})

	Describe(".GetTop", func() {
		key := ed25519.NewKeyFromIntSeed(1)

		It("should return top n tx", func() {
			n := 2
			tx1 := txns.NewCoinTransferTx(1, key.Addr(), key, "10", "1", 0)
			tx2 := txns.NewCoinTransferTx(2, key.Addr(), key, "25", "1", 0)
			mockMempoolReactor.EXPECT().GetTop(n).Return([]types.BaseTx{tx1, tx2})
			res := m.GetTop(n)
			Expect(res).To(HaveLen(2))
		})
	})

	Describe(".GetPushPoolSize", func() {
		It("should return push pool size", func() {
			mockPushPool.EXPECT().Len().Return(123)
			size := m.GetPushPoolSize()
			Expect(size).To(Equal(123))
		})
	})
})
