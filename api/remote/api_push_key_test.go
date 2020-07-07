package remote

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("PushKeyAPI", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetPushKey", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"id and blockHeight should be passed to PushKeyModule#Get": {
				params:     map[string]string{"id": "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", "height": "1"},
				resp:       `{"address":"maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc","pubKey":"49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().
						Get("push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", uint64(1)).
						Return(util.Map{
							"pubKey":  "49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ",
							"address": "maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc"})
					modules.PushKey = mockPushKeyModule
				},
			},
		}, api.GetPushKey)
	})

	Describe(".GetPushKeyOwnerNonce", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return expected nonce": {
				params:     map[string]string{"id": "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", "height": "1"},
				resp:       `{"nonce":"1000"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().
						GetAccountOfOwner("push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", uint64(1)).
						Return(util.Map{"nonce": "1000"})
					modules.PushKey = mockPushKeyModule
				},
			},
		}, api.GetPushKeyOwnerNonce)
	})
})
