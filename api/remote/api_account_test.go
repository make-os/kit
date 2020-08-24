package remote

import (
	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/pkgs/logger"
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

	Describe(".GetAccountNonce", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return nonce": {
				params:     map[string]string{"address": "os1z"},
				resp:       `{"nonce":"100"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockAcctModule := mocks.NewMockUserModule(ctrl)
					mockAcctModule.EXPECT().GetNonce("os1z", uint64(0)).Return("100")
					modules.User = mockAcctModule
				},
			},
			"should pass height to UserModule.GetNonce if 'height' param is set": {
				params:     map[string]string{"address": "os1z", "height": "100"},
				resp:       `{"nonce":"100"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockAcctModule := mocks.NewMockUserModule(ctrl)
					mockAcctModule.EXPECT().GetNonce("os1z", uint64(100)).Return("100")
					modules.User = mockAcctModule
				},
			},
		}, api.GetAccountNonce)
	})

	Describe(".GetAccount", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return account if found": {
				params:     map[string]string{"address": "os1zt", "height": "100"},
				resp:       `{"balance":"1200"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					acct := util.Map{"balance": "1200"}
					mockAcctModule := mocks.NewMockUserModule(ctrl)
					mockAcctModule.EXPECT().
						GetAccount("os1zt", uint64(100)).Return(acct)
					modules.User = mockAcctModule
				},
			},
		}, api.GetAccount)
	})
})
