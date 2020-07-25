package rpc

import (
	"github.com/golang/mock/gomock"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/modules/types"
	"github.com/themakeos/lobe/rpc"
	"github.com/themakeos/lobe/util"

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

	Describe(".registerPushKey", func() {
		mods := &types.Modules{}
		api := &PushKeyAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params:     "{}",
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a map", Data: ""},
			},
			"should return code=200 on success": {
				params:     map[string]interface{}{"key": "value"},
				result:     util.Map{"address": "push1abc", "hash": "0x123"},
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().Register(tc.params).Return(util.Map{"address": "push1abc", "hash": "0x123"})
					mods.PushKey = mockPushKeyModule
				},
			},
		}, api.registerPushKey)
	})
})
