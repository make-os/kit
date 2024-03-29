package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("NamespaceModule", func() {
	var m *modules.NamespaceModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockRemoteSrv *mocks.MockRemoteServer
	var mockLogic *mocks.MockLogic
	var mockNSKeeper *mocks.MockNamespaceKeeper
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockMempoolReactor *mocks.MockMempoolReactor

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockRemoteSrv = mocks.NewMockRemoteServer(ctrl)
		mockNSKeeper = mocks.NewMockNamespaceKeeper(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockSysKeeper = mocks.NewMockSystemKeeper(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockLogic.EXPECT().NamespaceKeeper().Return(mockNSKeeper).AnyTimes()
		mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		m = modules.NewNamespaceModule(mockService, mockRemoteSrv, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceNS)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".Lookup", func() {
		It("should return nil if namespace does not exist", func() {
			mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("name"), uint64(0)).Return(state.BareNamespace())
			res := m.Lookup("name", 0)
			Expect(res).To(BeNil())
		})

		It("should panic if unable to get latest block info", func() {
			ns := state.BareNamespace()
			ns.Owner = "r/repo"
			mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("name"), uint64(0)).Return(ns)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Lookup("name", 0)
			})
		})

		It("should set 'expired'=true and 'grace'=true if namespace expiredAt=100 and graceEndAt=200 when chainHeight=101", func() {
			ns := state.BareNamespace()
			ns.Owner = "r/repo"
			ns.ExpiresAt = 100
			ns.GraceEndAt = 200
			mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("name"), uint64(0)).Return(ns)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 101}, nil)
			res := m.Lookup("name", 0)
			Expect(res).ToNot(BeNil())
			Expect(res["expired"]).To(BeTrue())
			Expect(res["grace"]).To(BeTrue())
		})

		It("should set 'expired'=true and 'grace'=false if namespace expiredAt=100 and graceEndAt=200 when chainHeight=200", func() {
			ns := state.BareNamespace()
			ns.Owner = "r/repo"
			ns.ExpiresAt = 100
			ns.GraceEndAt = 200
			mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("name"), uint64(0)).Return(ns)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 200}, nil)
			res := m.Lookup("name", 0)
			Expect(res).ToNot(BeNil())
			Expect(res["expired"]).To(BeTrue())
			Expect(res["grace"]).To(BeFalse())
		})
	})

	Describe(".GetTarget", func() {
		It("should panic if unable to get path target", func() {
			mockNSKeeper.EXPECT().GetTarget("namespace/domain", uint64(0)).Return("", fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetTarget("namespace/domain", 0)
			})
		})

		It("should target on success", func() {
			mockNSKeeper.EXPECT().GetTarget("namespace/domain", uint64(0)).Return("r/repo", nil)
			target := m.GetTarget("namespace/domain", 0)
			Expect(target).To(Equal("r/repo"))
		})
	})

	Describe(".Register", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"name": struct{}{}}
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"name": "ns1"}
			res := m.Register(params, key, payloadOnly)
			Expect(res).To(HaveKey("name"))
			Expect(res["name"]).To(Equal(crypto.MakeNamespaceHash("ns1")))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeNamespaceRegister)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("domains"),
				HaveKey("to"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "ns1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"name": "ns1"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Register(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".UpdateDomain", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"name": struct{}{}}
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UpdateDomain(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"name": "ns1"}
			res := m.UpdateDomain(params, key, payloadOnly)
			Expect(res).To(HaveKey("name"))
			Expect(res["name"]).To(Equal(crypto.MakeNamespaceHash("ns1")))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeNamespaceDomainUpdate)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("domains"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "ns1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UpdateDomain(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"name": "ns1"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.UpdateDomain(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})
})
