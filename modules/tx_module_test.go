package modules_test

import (
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	crypto2 "gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
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

	Describe(".ConsoleOnlyMode", func() {
		It("should return false", func() {
			Expect(m.ConsoleOnlyMode()).To(BeFalse())
		})
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

	Describe(".SendCoin()", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"type": struct{}{}}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:type, msg:invalid value type: has struct {}, wants int", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendCoin(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			params := tx.ToMap()
			res := m.SendCoin(params, key, payloadOnly)
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res).To(And(
				HaveKey("nonce"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("to"),
				HaveKey("timestamp"),
				HaveKey("fee"),
				HaveKey("sig"),
				HaveKey("value"),
			))
		})

		It("should return panic if unable to add tx to mempool", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			params := tx.ToMap()
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendCoin(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			params := tx.ToMap()
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.SendCoin(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Get", func() {
		It("should panic if transaction hash is not valid", func() {
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "invalid transaction hash", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("000_invalid_hash")
			})
		})

		It("should panic if transaction does not exist", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(nil, types.ErrTxNotFound)
			err := &util.StatusError{Code: "tx_not_found", HttpCode: 404, Msg: "transaction not found", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get(hash)
			})
		})

		It("should panic if unable to get transaction", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			hash := tx.GetID()
			mockTxKeeper.EXPECT().GetTx(util.MustFromHex(hash)).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
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
		It("should panic if unable to decoded parameter", func() {
			params := map[string]interface{}{"type": struct{}{}}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:type, msg:invalid value type: has struct {}, wants int", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(params)
			})
		})

		It("should panic when unable to add tx to mempool", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendPayload(tx.ToMap())
			})
		})

		It("should panic when unable to add tx to pool due to BadFieldError", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			bfe := util.FieldError("field", "error")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, bfe)
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: "field"}
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
