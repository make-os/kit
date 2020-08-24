package remote

import (
	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
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
				params:     map[string]string{"id": "pk1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", "height": "1"},
				resp:       `{"address":"os1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc","pubKey":"49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().
						Get("pk1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", uint64(1)).
						Return(util.Map{
							"pubKey":  "49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ",
							"address": "os1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc"})
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
				params:     map[string]string{"id": "pk1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", "height": "1"},
				resp:       `{"nonce":"1000"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().
						GetAccountOfOwner("pk1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", uint64(1)).
						Return(util.Map{"nonce": "1000"})
					modules.PushKey = mockPushKeyModule
				},
			},
		}, api.GetPushKeyOwnerNonce)
	})

	Describe(".Register", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testPostRequestCases(map[string]TestCase{
			"should return error when params could not be json decoded": {
				paramsRaw:  []byte("{"),
				resp:       `{"error":{"code":"0","msg":"malformed body"}}`,
				statusCode: 400,
			},
			"should return call module's Register method and return 201 on success": {
				paramsRaw:  []byte("{}"),
				resp:       `{"hash":"0x123"}`,
				statusCode: 201,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().Register(gomock.Any()).Return(util.Map{"hash": "0x123"})
					modules.PushKey = mockPushKeyModule
				},
			},
		}, api.RegisterPushKey)
	})
})
