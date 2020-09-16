package modules_test

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	crypto2 "github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/keystore"
	"github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/mocks"
	mocks2 "github.com/make-os/lobe/mocks/rpc"
	"github.com/make-os/lobe/modules"
	"github.com/make-os/lobe/testutil"
	types2 "github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/api"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("UserModule", func() {
	var m *modules.UserModule
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockKeystore *mocks.MockKeystore
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockSysKeeper *mocks.MockSystemKeeper
	var pk = crypto2.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockKeystore = mocks.NewMockKeystore(ctrl)
		mockAcctKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockSysKeeper = mocks.NewMockSystemKeeper(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
		mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
		m = modules.NewUserModule(cfg, mockKeystore, mockService, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			mockKeystore.EXPECT().List().Return(nil, nil)
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceUser)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".GetKeys()", func() {
		It("should panic when unable to get a list of local accounts", func() {
			mockKeystore.EXPECT().List().Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetKeys()
			})
		})

		It("should return address of accounts on success", func() {
			keys := []types.StoredKey{&keystore.StoredKey{UserAddress: "addr1"}, &keystore.StoredKey{UserAddress: "addr2"}}
			mockKeystore.EXPECT().List().Return(keys, nil)
			accts := m.GetKeys()
			Expect(accts).To(HaveLen(2))
			Expect(accts).To(And(ContainElement("addr1"), ContainElement("addr2")))
		})
	})

	Describe(".GetKey", func() {
		It("should panic when address is not provided", func() {
			err := &util.ReqError{Code: "addr_required", HttpCode: 400, Msg: "address is required", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPrivKey("", "pass")
			})
		})

		It("should panic when no key match the given address", func() {
			mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(nil, types2.ErrAccountUnknown)
			err := &util.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPrivKey("addr1", "pass")
			})
		})

		It("should panic when unable to fetch key matching the given address", func() {
			mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPrivKey("addr1", "pass")
			})
		})

		When("key is unprotected", func() {
			It("should panic when failed to unlock key", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(true)
				mockKey.EXPECT().Unlock(keystore.DefaultPassphrase).Return(fmt.Errorf("unlock error"))
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "unlock error", Field: "passphrase"}
				assert.PanicsWithError(GinkgoT(), err.Error(), func() {
					m.GetPrivKey("addr1", "pass")
				})
			})

			It("should return private key when key unlock is successful", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(true)
				mockKey.EXPECT().Unlock(keystore.DefaultPassphrase).Return(nil)
				mockKey.EXPECT().GetKey().Return(pk)
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				res := m.GetPrivKey("addr1", "pass")
				Expect(res).To(Equal(pk.PrivKey().Base58()))
			})
		})

		When("key is protected", func() {
			It("should ask for passphrase and panic when key unlock failed", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(false)
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				mockKeystore.EXPECT().AskForPasswordOnce().Return("passphrase", nil)
				mockKey.EXPECT().Unlock("passphrase").Return(fmt.Errorf("unlock error"))
				err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "unlock error", Field: "passphrase"}
				assert.PanicsWithError(GinkgoT(), err.Error(), func() {
					m.GetPrivKey("addr1")
				})
			})

			It("should ask for passphrase and return private key when key unlock is successful", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(false)
				mockKeystore.EXPECT().AskForPasswordOnce().Return("passphrase", nil)
				mockKey.EXPECT().Unlock("passphrase").Return(nil)
				mockKey.EXPECT().GetKey().Return(pk)
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				res := m.GetPrivKey("addr1")
				Expect(res).To(Equal(pk.PrivKey().Base58()))
			})
		})
	})

	Describe(".GetPublicKey", func() {
		It("should panic when address is not provided", func() {
			err := &util.ReqError{Code: "addr_required", HttpCode: 400, Msg: "address is required", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPublicKey("", "pass")
			})
		})

		It("should panic when no key match the given address", func() {
			mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(nil, types2.ErrAccountUnknown)
			err := &util.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPublicKey("addr1", "pass")
			})
		})

		It("should panic when unable to fetch key matching the given address", func() {
			mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPublicKey("addr1", "pass")
			})
		})

		When("key is unprotected", func() {
			It("should panic when failed to unlock key", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(true)
				mockKey.EXPECT().Unlock(keystore.DefaultPassphrase).Return(fmt.Errorf("unlock error"))
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "unlock error", Field: "passphrase"}
				assert.PanicsWithError(GinkgoT(), err.Error(), func() {
					m.GetPublicKey("addr1", "pass")
				})
			})

			It("should return private key when key unlock is successful", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(true)
				mockKey.EXPECT().Unlock(keystore.DefaultPassphrase).Return(nil)
				mockKey.EXPECT().GetKey().Return(pk)
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				res := m.GetPublicKey("addr1", "pass")
				Expect(res).To(Equal(pk.PubKey().Base58()))
			})
		})

		When("key is protected", func() {
			It("should ask for passphrase and panic when key unlock failed", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(false)
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				mockKeystore.EXPECT().AskForPasswordOnce().Return("passphrase", nil)
				mockKey.EXPECT().Unlock("passphrase").Return(fmt.Errorf("unlock error"))
				err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "unlock error", Field: "passphrase"}
				assert.PanicsWithError(GinkgoT(), err.Error(), func() {
					m.GetPublicKey("addr1")
				})
			})

			It("should ask for passphrase and return private key when key unlock is successful", func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().IsUnprotected().Return(false)
				mockKeystore.EXPECT().AskForPasswordOnce().Return("passphrase", nil)
				mockKey.EXPECT().Unlock("passphrase").Return(nil)
				mockKey.EXPECT().GetKey().Return(pk)
				mockKeystore.EXPECT().GetByIndexOrAddress("addr1").Return(mockKey, nil)
				res := m.GetPublicKey("addr1")
				Expect(res).To(Equal(pk.PubKey().Base58()))
			})
		})
	})

	Describe(".GetNonce", func() {
		It("should panic when account does not exist", func() {
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(state.BareAccount())
			err := &util.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetNonce("addr1")
			})
		})

		It("should return account nonce on success", func() {
			acct := state.BareAccount()
			acct.Nonce = 100
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			nonce := m.GetNonce("addr1")
			Expect(nonce).To(Equal("100"))
		})
	})

	Describe(".GetAccount", func() {
		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockUserClient := mocks2.NewMockUser(ctrl)
			mockClient.EXPECT().User().Return(mockUserClient)
			m.Client = mockClient

			mockUserClient.EXPECT().Get("os1abc", uint64(1)).Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetAccount("os1abc", 1)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockUserClient := mocks2.NewMockUser(ctrl)
			mockClient.EXPECT().User().Return(mockUserClient)
			m.Client = mockClient
			mockUserClient.EXPECT().Get("os1abc", uint64(1)).Return(&api.ResultAccount{}, nil)
			assert.NotPanics(GinkgoT(), func() {
				m.GetAccount("os1abc", 1)
			})
		})

		It("should panic when account does not exist", func() {
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(state.BareAccount())
			err := &util.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetAccount("addr1")
			})
		})

		It("should return account on success", func() {
			acct := state.BareAccount()
			acct.Balance = "100.22"
			acct.Nonce = 100
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			res := m.GetAccount("addr1")
			Expect(res).To(Equal(util.Map{
				"balance":             util.String("100.22"),
				"nonce":               util.UInt64(100),
				"delegatorCommission": float64(0),
			}))
		})

		It("should return account with stakes info if account has a non-empty stakes field", func() {
			acct := state.BareAccount()
			acct.Balance = "100.22"
			acct.Nonce = 100
			acct.Stakes.Add(state.StakeTypeHost, "10", 1000)
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			res := m.GetAccount("addr1")
			Expect(res).To(Equal(util.Map{
				"balance":             util.String("100.22"),
				"nonce":               util.UInt64(100),
				"delegatorCommission": float64(0),
				"stakes": map[string]interface{}{
					"s0": map[string]interface{}{"value": util.String("10"), "unbondHeight": util.UInt64(1000)},
				},
			}))
		})
	})

	Describe(".GetAvailableBalance", func() {
		It("should panic when account does not exist", func() {
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(state.BareAccount())
			err := &util.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetAvailableBalance("addr1")
			})
		})

		It("should return 100 when balance is 100 and no staked balance", func() {
			acct := state.BareAccount()
			acct.Balance = "100"
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)
			res := m.GetAvailableBalance("addr1")
			Expect(res).To(Equal("100"))
		})

		It("should return 90 when balance is 100 and staked balance is 10", func() {
			acct := state.BareAccount()
			acct.Balance = "100"
			acct.Stakes.Add(state.StakeTypeHost, "10", 0)
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)
			res := m.GetAvailableBalance("addr1")
			Expect(res).To(Equal("90"))
		})
	})

	Describe(".GetStakedBalance()", func() {
		It("should panic when account does not exist", func() {
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(state.BareAccount())
			err := &util.ReqError{Code: "account_not_found", HttpCode: 404, Msg: "account not found", Field: "address"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetStakedBalance("addr1")
			})
		})

		It("should return 0 when no staked balance", func() {
			acct := state.BareAccount()
			acct.Balance = "100"
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)
			res := m.GetStakedBalance("addr1")
			Expect(res).To(Equal("0"))
		})

		It("should return 10 when staked balance is 10", func() {
			acct := state.BareAccount()
			acct.Balance = "100"
			acct.Stakes.Add(state.StakeTypeHost, "10", 0)
			mockAcctKeeper.EXPECT().Get(identifier.Address("addr1")).Return(acct)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)
			res := m.GetStakedBalance("addr1")
			Expect(res).To(Equal("10"))
		})
	})

	Describe(".GetValidator", func() {
		It("should not include private key if 'includePrivKey' argument is set", func() {
			res := m.GetValidator()
			Expect(res).To(And(
				HaveKey("address"),
				HaveKey("tmAddr"),
				HaveKey("pubkey"),
			))
			Expect(res).ToNot(HaveKey("privkey"))
		})

		It("should not include private key if 'includePrivKey' is set to 'false'", func() {
			res := m.GetValidator()
			Expect(res).To(And(
				HaveKey("address"),
				HaveKey("tmAddr"),
				HaveKey("pubkey"),
			))
			Expect(res).ToNot(HaveKey("privkey"))
		})

		It("should include private key if 'includePrivKey' is set to 'true'", func() {
			res := m.GetValidator(true)
			Expect(res).To(And(
				HaveKey("address"),
				HaveKey("tmAddr"),
				HaveKey("pubkey"),
				HaveKey("privkey"),
			))
		})
	})

	Describe(".SetCommission", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"commission": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'commission' expected type 'util.String', got unconvertible type 'struct {}'", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SetCommission(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"commission": 90.2}
			res := m.SetCommission(params, key, true)
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("commission"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeSetDelegatorCommission)))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"commission": 90.2}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SetCommission(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"commission": 90.2}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.SetCommission(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".SendCoin()", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"type": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'type' expected type 'types.TxCode', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendCoin(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			params := tx.ToMap()
			res := m.SendCoin(params, "", true)
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

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockUserClient := mocks2.NewMockUser(ctrl)
			mockClient.EXPECT().User().Return(mockUserClient)
			m.Client = mockClient

			mockUserClient.EXPECT().Send(gomock.Any()).Return(nil, fmt.Errorf("error"))
			params := map[string]interface{}{"value": "10"}
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SendCoin(params)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockUserClient := mocks2.NewMockUser(ctrl)
			mockClient.EXPECT().User().Return(mockUserClient)
			m.Client = mockClient
			params := map[string]interface{}{"value": "10"}

			mockUserClient.EXPECT().Send(gomock.Any()).Return(&api.ResultHash{}, nil)
			assert.NotPanics(GinkgoT(), func() {
				m.SendCoin(params)
			})
		})

		It("should panic if unable to add tx to mempool", func() {
			tx := txns.NewCoinTransferTx(1, pk.Addr(), pk, "1", "1", time.Now().Unix())
			params := tx.ToMap()
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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

})
