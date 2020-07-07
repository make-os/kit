package remote

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetAccountNonce", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return nonce": {
				params:     map[string]string{"address": "maker1z"},
				resp:       `{"nonce":"100"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockAcctModule := mocks.NewMockAccountModule(ctrl)
					mockAcctModule.EXPECT().GetNonce("maker1z", uint64(0)).Return("100")
					modules.Account = mockAcctModule
				},
			},
			"should pass height to AccountModule.GetNonce if 'height' param is set": {
				params:     map[string]string{"address": "maker1z", "height": "100"},
				resp:       `{"nonce":"100"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockAcctModule := mocks.NewMockAccountModule(ctrl)
					mockAcctModule.EXPECT().GetNonce("maker1z", uint64(100)).Return("100")
					modules.Account = mockAcctModule
				},
			},
		}, api.GetAccountNonce)
	})

	Describe(".GetAccount", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return account if found": {
				params:     map[string]string{"address": "maker1zt", "height": "100"},
				resp:       `{"balance":"1200"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					acct := util.Map{"balance": "1200"}
					mockAcctModule := mocks.NewMockAccountModule(ctrl)
					mockAcctModule.EXPECT().
						GetAccount("maker1zt", uint64(100)).Return(acct)
					modules.Account = mockAcctModule
				},
			},
		}, api.GetAccount)
	})
})
