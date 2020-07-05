package api

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
)

var _ = Describe("AccountLocal", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".listAccounts", func() {
		mods := &types.Modules{}
		api := &LocalAccountAPI{mods}
		testCases(map[string]*TestCase{
			"should return slice of addresses": {
				params: nil,
				result: map[string]interface{}{
					"accounts": []string{"addr1", "addr2"},
				},
				mocker: func(tp *TestCase) {
					mockAcctMod := mocks.NewMockAccountModule(ctrl)
					mockAcctMod.EXPECT().ListLocalAccounts().Return([]string{"addr1", "addr2"})
					mods.Account = mockAcctMod
				},
			},
		}, api.listAccounts)
	})
})
