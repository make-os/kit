package modules_test

import (
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	types2 "gitlab.com/makeos/lobe/api/types"
	crypto2 "gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/mocks"
	mocks2 "gitlab.com/makeos/lobe/mocks/rpc"
	"gitlab.com/makeos/lobe/modules"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

var _ = Describe("TxModule", func() {
	var m *modules.TxModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockTxKeeper *mocks.MockTxKeeper
	var pk = crypto2.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockTxKeeper = mocks.NewMockTxKeeper(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().TxKeeper().Return(mockTxKeeper).AnyTimes()
		m = modules.NewTxModule(mockService, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceTx)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".Get", func() {
		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().GetTransaction("0x123").Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("0x123")
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().GetTransaction("0x123").Return(map[string]interface{}{}, nil)
			assert.NotPanics(GinkgoT(), func() {
				m.Get("0x123")
			})
		})

		It("should panic if transaction hash is not valid", func() {
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "invalid transaction hash", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("000_invalid_hash")
			})
		})

		It("should panic if transaction does not exist", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(nil, types.ErrTxNotFound)
			err := &util.ReqError{Code: "tx_not_found", HttpCode: 404, Msg: "transaction not found", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get(hash)
			})
		})

		It("should panic if unable to get transaction", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get(hash)
			})
		})

		It("should return tx successfully if tx exist", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(tx, nil)
			res := m.Get(hash)
			Expect(util.Map(util.ToMap(tx))).To(Equal(res))
		})
	})

	Describe(".SendPayload", func() {
		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			payload := map[string]interface{}{"type": 1}
			mockClient.EXPECT().SendTxPayload(payload).Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(payload)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			payload := map[string]interface{}{"type": 1}
			mockClient.EXPECT().SendTxPayload(payload).Return(&types2.HashResponse{}, nil)
			assert.NotPanics(GinkgoT(), func() {
				m.SendPayload(payload)
			})
		})

		It("should panic if unable to decoded parameter", func() {
			params := map[string]interface{}{"type": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'type' expected type 'types.TxCode', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(params)
			})
		})

		It("should panic when unable to add tx to mempool", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(tx.ToMap())
			})
		})

		It("should panic when unable to add tx to pool due to badFieldError", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			bfe := util.FieldError("field", "error")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, bfe)
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: "field"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(tx.ToMap())
			})
		})

		It("should return hash on success", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(tx.GetHash(), nil)
			res := m.SendPayload(tx.ToMap())
			Expect(res).ToNot(BeEmpty())
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(tx.GetHash()))
		})
	})
})
