package rpc

import (
	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".getNonce", func() {
		mods := &types.Modules{}
		api := &UserAPI{mods}
		testCases(map[string]*TestCase{
			"when nonce is successfully returned": {
				params: map[string]interface{}{"address": "addr1"},
				result: map[string]interface{}{"nonce": "100"},
				mocker: func(tp *TestCase) {
					mockAcctMod := mocks.NewMockUserModule(ctrl)
					mockAcctMod.EXPECT().GetNonce("addr1", uint64(0)).Return("100")
					mods.User = mockAcctMod
				},
			},
		}, api.getNonce)
	})

	Describe(".getAccount", func() {
		mods := &types.Modules{}
		api := &UserAPI{mods}
		testCases(map[string]*TestCase{
			"when account is successfully returned": {
				params: map[string]interface{}{"address": "addr1"},
				result: map[string]interface{}{"balance": "100"},
				mocker: func(tp *TestCase) {
					mockUserModule := mocks.NewMockUserModule(ctrl)
					mockUserModule.EXPECT().GetAccount("addr1", uint64(0)).Return(util.Map{
						"balance": "100",
					})
					mods.User = mockUserModule
				},
			},
		}, api.getAccount)
	})

	Describe(".sendCoin", func() {
		mods := &types.Modules{}
		api := &UserAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params:     "{}",
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a map", Data: ""},
			},
			"should return code=200 on success": {
				params:     map[string]interface{}{"value": "100"},
				result:     util.Map{"hash": "0x123"},
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockUserModule := mocks.NewMockUserModule(ctrl)
					mockUserModule.EXPECT().SendCoin(tc.params).Return(util.Map{"hash": "0x123"})
					mods.User = mockUserModule
				},
			},
		}, api.sendCoin)
	})
})
