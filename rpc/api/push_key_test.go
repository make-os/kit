package api

import (
	"github.com/golang/mock/gomock"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("PushKey", func() {
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".find", func() {
		mods := &types.Modules{}
		api := &PushKeyAPI{mods}
		testCases(map[string]*TestCase{
			"when push key is successfully returned": {
				params: map[string]interface{}{"id": "push1_abc"},
				result: util.Map{
					"pubKey":  key.PubKey().ToPublicKey(),
					"address": "addr1",
				},
				mocker: func(tp *TestCase) {
					mockPushKeyMod := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyMod.EXPECT().Get("push1_abc", uint64(0)).Return(util.Map{
						"pubKey":  key.PubKey().ToPublicKey(),
						"address": "addr1",
					})
					mods.PushKey = mockPushKeyMod
				},
			},
		}, api.find)
	})

	Describe(".getOwner", func() {
		mods := &types.Modules{}
		api := &PushKeyAPI{mods}
		testCases(map[string]*TestCase{
			"when account is successfully returned": {
				params: map[string]interface{}{"id": "push1_abc"},
				result: util.Map{"balance": "100", "nonce": 10, "delegatorCommission": 23},
				mocker: func(tp *TestCase) {
					mockPushKeyMod := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyMod.EXPECT().GetAccountOfOwner("push1_abc", uint64(0)).Return(util.Map{
						"balance":             "100",
						"nonce":               10,
						"delegatorCommission": 23,
					})
					mods.PushKey = mockPushKeyMod
				},
			},
		}, api.getOwner)
	})
})
