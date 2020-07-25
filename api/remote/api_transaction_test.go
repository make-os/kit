package remote

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/lobe/mocks"
	"gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/pkgs/logger"
	"gitlab.com/makeos/lobe/util"
)

var _ = Describe("Transaction", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".SendTxPayload", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testPostRequestCases(map[string]TestCase{
			"should return error when unable to decode body to json": {
				paramsRaw:  []byte("{"),
				resp:       `{"error":{"code":"0","msg":"malformed body"}}`,
				statusCode: 400,
			},
			"should send payload": {
				params:     map[string]string{},
				resp:       `{"hash":"0x000000"}`,
				statusCode: 201,
				mocker: func(tc *TestCase) {
					mockTxModule := mocks.NewMockTxModule(ctrl)
					mockTxModule.EXPECT().SendPayload(make(map[string]interface{})).Return(util.Map{"hash": "0x000000"})
					modules.Tx = mockTxModule
				},
			},
		}, api.SendTxPayload)
	})

	Describe(".GetTransaction", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return result": {
				params:     map[string]string{"hash": "0x123"},
				resp:       `{"value":"10.4"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockTxModule := mocks.NewMockTxModule(ctrl)
					mockTxModule.EXPECT().Get("0x123").Return(map[string]interface{}{"value": "10.4"})
					modules.Tx = mockTxModule
				},
			},
		}, api.GetTransaction)
	})
})
