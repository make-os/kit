package remote

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/lobe/mocks"
	"gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/pkgs/logger"
	"gitlab.com/makeos/lobe/util"
)

var _ = Describe("User", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".SendCoin", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testPostRequestCases(map[string]TestCase{
			"should return error when unable to decode body to json": {
				paramsRaw:  []byte("{"),
				resp:       `{"error":{"code":"0","msg":"malformed body"}}`,
				statusCode: 400,
			},
			"should request and call SendCoin module method": {
				params:     map[string]string{},
				resp:       `{"hash":"0x000000"}`,
				statusCode: 201,
				mocker: func(tc *TestCase) {
					mockUserModule := mocks.NewMockUserModule(ctrl)
					mockUserModule.EXPECT().SendCoin(make(map[string]interface{})).Return(util.Map{"hash": "0x000000"})
					modules.User = mockUserModule
				},
			},
		}, api.SendCoin)
	})
})
