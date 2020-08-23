package rpc

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/modules/types"
)

var _ = Describe("AccountLocal", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".getKeys", func() {
		mods := &types.Modules{}
		api := &LocalAccountAPI{mods}
		testCases(map[string]*TestCase{
			"should return slice of addresses": {
				params: nil,
				result: map[string]interface{}{
					"accounts": []string{"addr1", "addr2"},
				},
				mocker: func(tp *TestCase) {
					mockAcctMod := mocks.NewMockUserModule(ctrl)
					mockAcctMod.EXPECT().GetKeys().Return([]string{"addr1", "addr2"})
					mods.User = mockAcctMod
				},
			},
		}, api.listAccounts)
	})
})
