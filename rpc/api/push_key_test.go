package api

import (
	"github.com/golang/mock/gomock"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/rpc"
)

var _ = Describe("PushKey", func() {
	var ctrl *gomock.Controller
	var pushApi *PushKeyAPI
	var mods *types.Modules

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mods = &types.Modules{}
		pushApi = &PushKeyAPI{mods}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".find", func() {
		var key = crypto.NewKeyFromIntSeed(1)
		testCases := map[string]*TestCase{
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
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := pushApi.find(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})

	Describe(".getOwner", func() {
		testCases := map[string]*TestCase{
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
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := pushApi.getOwner(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})
})
