package rpc

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller
	var acctApi *AccountAPI
	var mods *types.Modules

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mods = &types.Modules{}
		acctApi = &AccountAPI{mods}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".getNonce", func() {
		testCases := map[string]*TestCase{
			"when address is not provided": {
				params: map[string]interface{}{},
				err:    &rpc.Err{Code: "60001", Message: "address is required", Data: "address"},
			},
			"when address type is not string": {
				params: map[string]interface{}{"address": 222},
				err:    &rpc.Err{Code: "60001", Message: "wrong value type, want 'string', got string", Data: "address"},
			},
			"when nonce is successfully returned": {
				params: map[string]interface{}{"address": "addr1"},
				result: map[string]interface{}{"nonce": "100"},
				mocker: func(tp *TestCase) {
					mockAcctMod := mocks.NewMockAccountModule(ctrl)
					mockAcctMod.EXPECT().GetNonce("addr1", uint64(0)).Return("100")
					mods.Account = mockAcctMod
				},
			},
		}

		for tc, tp := range testCases {
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := acctApi.getNonce(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})

	Describe(".getAccount()", func() {
		testCases := map[string]*TestCase{
			"when address is not provided": {
				params: map[string]interface{}{},
				err:    &rpc.Err{Code: "60001", Message: "address is required", Data: "address"},
			},
			"when address type is not string": {
				params: map[string]interface{}{"address": 222},
				err:    &rpc.Err{Code: "60001", Message: "wrong value type, want 'string', got string", Data: "address"},
			},
			"when account is successfully returned": {
				params: map[string]interface{}{"address": "addr1"},
				result: map[string]interface{}{"balance": "100"},
				mocker: func(tp *TestCase) {
					mockAcctMod := mocks.NewMockAccountModule(ctrl)
					mockAcctMod.EXPECT().GetAccount("addr1", uint64(0)).Return(util.Map{
						"balance": "100",
					})
					mods.Account = mockAcctMod
				},
			},
		}

		for tc, tp := range testCases {
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := acctApi.getAccount(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})
})
