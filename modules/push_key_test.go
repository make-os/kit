package modules_test

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	crypto2 "github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	mocksrpc "github.com/make-os/kit/mocks/rpc"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("PushKeyModule", func() {
	var err error
	var cfg *config.AppConfig
	var m *modules.PushKeyModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper
	var mockAccountKeeper *mocks.MockAccountKeeper
	var pk = crypto2.NewKeyFromIntSeed(1)
	var mockClient *mocksrpc.MockClient
	var mockPushKeyClient *mocksrpc.MockPushKey

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mocksrpc.NewMockClient(ctrl)

		mockPushKeyClient = mocksrpc.NewMockPushKey(ctrl)
		mockClient.EXPECT().PushKey().Return(mockPushKeyClient).AnyTimes()

		mockService = mocks.NewMockService(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockPushKeyKeeper = mocks.NewMockPushKeyKeeper(ctrl)
		mockAccountKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().PushKeyKeeper().Return(mockPushKeyKeeper).AnyTimes()
		mockLogic.EXPECT().AccountKeeper().Return(mockAccountKeeper).AnyTimes()
		m = modules.NewPushKeyModule(cfg, mockService, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespacePushKey)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".Register", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"pubKey": struct{}{}}
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'pubKey[0]' expected type 'uint8', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"pubKey": pk.PubKey().Base58()}
			res := m.Register(params, key, true)
			Expect(res).To(HaveKey("pubKey"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRegisterPushKey)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("feeCap"),
				HaveKey("scopes"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockPushKeyClient.EXPECT().Register(gomock.Any()).Return(nil, fmt.Errorf("error"))
			m.Client = mockClient
			params := map[string]interface{}{"pubKey": pk.PubKey().Base58()}
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params, "", false)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockPushKeyClient.EXPECT().Register(gomock.Any()).Return(&api.ResultRegisterPushKey{}, nil)
			m.Client = mockClient
			params := map[string]interface{}{"pubKey": pk.PubKey().Base58()}
			assert.NotPanics(GinkgoT(), func() {
				m.Register(params, "", false)
			})
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"pubKey": pk.PubKey().Base58()}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params, "", false)
			})
		})

		It("should return tx hash and push key address on success", func() {
			params := map[string]interface{}{"pubKey": pk.PubKey().Base58()}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Register(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
			Expect(res).To(HaveKey("address"))
			Expect(res["address"]).To(Equal(pk.PushAddr().String()))
		})
	})

	Describe(".Update", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"id": struct{}{}}
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Update(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"id": pk.PushAddr().String()}
			res := m.Update(params, key, true)
			Expect(res).To(HaveKey("id"))
			Expect(res["id"]).To(Equal(pk.PushAddr().String()))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeUpDelPushKey)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("feeCap"),
				HaveKey("addScopes"),
				HaveKey("removeScopes"),
				HaveKey("delete"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": pk.PushAddr().String()}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Update(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": pk.PushAddr().String()}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Update(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Unregister", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"id": struct{}{}}
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Unregister(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"id": pk.PushAddr().String()}
			res := m.Unregister(params, key, true)
			Expect(res).To(HaveKey("id"))
			Expect(res["id"]).To(Equal(pk.PushAddr().String()))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeUpDelPushKey)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("feeCap"),
				HaveKey("addScopes"),
				HaveKey("removeScopes"),
				HaveKey("delete"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": pk.PushAddr().String()}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Unregister(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": pk.PushAddr().String()}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Unregister(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Get", func() {

		It("should panic when push key address is not provided", func() {
			id := ""
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "push key id is required", Field: "id"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Find(id, 0)
			})
		})

		It("should panic if unable to get push key", func() {
			id := "pk1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t"
			err := &errors.ReqError{Code: "push_key_not_found", HttpCode: 404, Msg: "push key not found", Field: ""}
			mockPushKeyKeeper.EXPECT().Get(id, uint64(0)).Return(state.BarePushKey())
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Find(id, 0)
			})
		})

		It("should return push key on success", func() {
			key := crypto2.NewKeyFromIntSeed(1)
			id := key.PushAddr().String()
			pushKey := state.BarePushKey()
			pushKey.PubKey = key.PubKey().ToPublicKey()
			pushKey.Address = key.Addr()
			mockPushKeyKeeper.EXPECT().Get(id, uint64(0)).Return(pushKey)
			res := m.Find(id, 0)
			Expect(res["pubKey"]).To(Equal(key.PubKey().ToPublicKey()))
			Expect(res["address"]).To(Equal(key.Addr()))
			Expect(res["feeUsed"]).To(Equal(util.String("0")))
			Expect(res["feeCap"]).To(Equal(util.String("0")))
			Expect(res["scopes"]).To(BeEmpty())
		})
	})

	Describe(".GetByAddress", func() {
		It("should return push key addresses", func() {
			key := crypto2.NewKeyFromIntSeed(1)
			expected := []string{"pk1_addr", "pk2_addr"}
			mockPushKeyKeeper.EXPECT().GetByAddress(key.Addr().String()).Return(expected)
			res := m.GetByAddress(key.Addr().String())
			Expect(res).To(Equal(expected))
		})
	})

	Describe(".GetAccountOfOwner", func() {
		key := crypto2.NewKeyFromIntSeed(1)
		id := key.PushAddr().String()
		BeforeEach(func() {
			pushKey := state.BarePushKey()
			pushKey.PubKey = key.PubKey().ToPublicKey()
			pushKey.Address = key.Addr()
			mockPushKeyKeeper.EXPECT().Get(id, uint64(1)).Return(pushKey).AnyTimes()
		})

		It("should panic when unable to get push key owner account", func() {
			mockAccountKeeper.EXPECT().Get(key.Addr(), uint64(1)).Return(state.NewBareAccount())
			err := &errors.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetAccountOfOwner(key.PushAddr().String(), 1)
			})
		})

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockPushKeyClient.EXPECT().GetOwner(key.PushAddr().String(), uint64(1)).Return(nil, fmt.Errorf("error"))
			m.Client = mockClient
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetAccountOfOwner(key.PushAddr().String(), 1)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockPushKeyClient.EXPECT().GetOwner(key.PushAddr().String(), uint64(1)).Return(&api.ResultAccount{}, nil)
			m.Client = mockClient
			assert.NotPanics(GinkgoT(), func() {
				m.GetAccountOfOwner(key.PushAddr().String(), 1)
			})
		})

		It("should return account on success", func() {
			acct := state.NewBareAccount()
			acct.Balance = "100"
			mockAccountKeeper.EXPECT().Get(key.Addr(), uint64(1)).Return(acct)
			res := m.GetAccountOfOwner(key.PushAddr().String(), 1)
			Expect(res["delegatorCommission"]).To(Equal(float64(0)))
			Expect(res["balance"]).To(Equal(util.String("100")))
			Expect(res["nonce"]).To(Equal(util.UInt64(0)))
			Expect(res["stakes"]).To(Equal(map[string]interface{}{}))
		})
	})
})
