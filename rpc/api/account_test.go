package api

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
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

	Describe(".getNonce", func() {
		mods := &types.Modules{}
		api := &AccountAPI{mods}
		testCases(map[string]*TestCase{
			"when nonce is successfully returned": {
				params: map[string]interface{}{"address": "addr1"},
				result: map[string]interface{}{"nonce": "100"},
				mocker: func(tp *TestCase) {
					mockAcctMod := mocks.NewMockAccountModule(ctrl)
					mockAcctMod.EXPECT().GetNonce("addr1", uint64(0)).Return("100")
					mods.Account = mockAcctMod
				},
			},
		}, api.getNonce)
	})

	Describe(".getAccount", func() {
		mods := &types.Modules{}
		api := &AccountAPI{mods}
		testCases(map[string]*TestCase{
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
		}, api.getAccount)
	})
})
