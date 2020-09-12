package modules_test

import (
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	types2 "github.com/make-os/lobe/api/types"
	crypto2 "github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/mocks"
	mocks2 "github.com/make-os/lobe/mocks/rpc"
	mocks3 "github.com/make-os/lobe/mocks/rpc-client"
	"github.com/make-os/lobe/modules"
	types3 "github.com/make-os/lobe/remote/push/types"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
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
			mockTxClient := mocks3.NewMockTx(ctrl)
			mockClient.EXPECT().Tx().Return(mockTxClient)
			m.AttachedClient = mockClient

			mockTxClient.EXPECT().Get("0x123").Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("0x123")
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockTxClient := mocks3.NewMockTx(ctrl)
			mockClient.EXPECT().Tx().Return(mockTxClient)
			m.AttachedClient = mockClient

			mockTxClient.EXPECT().Get("0x123").Return(&types2.GetTxResponse{}, nil)
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

		It("should panic if unable to get transaction from tx index", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get(hash)
			})
		})

		It("should return result status=TxStatusInBlock and data=<tx> when transaction exists in tx index", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(tx, nil)
			res := m.Get(hash)
			Expect(res).To(HaveKey("status"))
			Expect(res["status"]).To(Equal(modules.TxStatusInBlock))
			Expect(res).To(HaveKey("data"))
			Expect(res["data"]).To(Equal(util.ToMap(tx)))
		})

		When("tx not found in tx index, check mempool", func() {
			It("should return result status=TxStatusInMempool and data=<tx> when transaction exists in the mempool", func() {
				tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
				hash := tx.GetID()
				mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(nil, types.ErrTxNotFound)
				mockMempoolReactor.EXPECT().GetTx(hash).Return(tx)
				res := m.Get(hash)
				Expect(res).To(HaveKey("status"))
				Expect(res["status"]).To(Equal(modules.TxStatusInMempool))
				Expect(res).To(HaveKey("data"))
				Expect(res["data"]).To(Equal(util.ToMap(tx)))
			})
		})

		When("tx not found in tx index and mempool, check pushpool", func() {
			It("should return result status=TxStatusInPushpool and data=<note> when transaction exists in the pushpool", func() {
				note := &types3.Note{RepoName: "repo1"}
				hash := note.ID()
				mockTxKeeper.EXPECT().GetTx(note.ID().Bytes()).Return(nil, types.ErrTxNotFound)
				mockMempoolReactor.EXPECT().GetTx(hash.String()).Return(nil)

				mockRemoteSrv := mocks.NewMockRemoteServer(ctrl)
				mockLogic.EXPECT().GetRemoteServer().Return(mockRemoteSrv)
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockRemoteSrv.EXPECT().GetPushPool().Return(mockPushPool)
				mockPushPool.EXPECT().Get(hash.HexStr()).Return(note)

				res := m.Get(hash.HexStr())
				Expect(res).To(HaveKey("status"))
				Expect(res["status"]).To(Equal(modules.TxStatusInPushpool))
				Expect(res).To(HaveKey("data"))
				Expect(res["data"]).To(Equal(util.ToBasicMap(note)))
			})
		})

		When("tx not found in tx index, mempool and pushpool", func() {
			It("should panic", func() {
				note := &types3.Note{RepoName: "repo1"}
				hash := note.ID()
				mockTxKeeper.EXPECT().GetTx(note.ID().Bytes()).Return(nil, types.ErrTxNotFound)
				mockMempoolReactor.EXPECT().GetTx(hash.String()).Return(nil)

				mockRemoteSrv := mocks.NewMockRemoteServer(ctrl)
				mockLogic.EXPECT().GetRemoteServer().Return(mockRemoteSrv)
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockRemoteSrv.EXPECT().GetPushPool().Return(mockPushPool)
				mockPushPool.EXPECT().Get(hash.HexStr()).Return(nil)

				err := &util.ReqError{Code: "tx_not_found", HttpCode: 404, Msg: "transaction not found", Field: "hash"}
				assert.PanicsWithError(GinkgoT(), err.Error(), func() {
					m.Get(hash.HexStr())
				})
			})
		})
	})

	Describe(".SendPayload", func() {
		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockTxClient := mocks3.NewMockTx(ctrl)
			mockClient.EXPECT().Tx().Return(mockTxClient)
			m.AttachedClient = mockClient

			payload := map[string]interface{}{"type": 1}
			mockTxClient.EXPECT().Send(payload).Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(payload)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockTxClient := mocks3.NewMockTx(ctrl)
			mockClient.EXPECT().Tx().Return(mockTxClient)
			m.AttachedClient = mockClient

			payload := map[string]interface{}{"type": 1}
			mockTxClient.EXPECT().Send(payload).Return(&types2.HashResponse{}, nil)
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
