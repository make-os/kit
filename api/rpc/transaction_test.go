package rpc

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/lobe/mocks"
	"gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/rpc"
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

	Describe(".sendPayload", func() {
		mods := &types.Modules{}
		api := &TransactionAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params: "{}",
				err:    &rpc.Err{Code: "60001", Message: "field:params, msg:wrong value type, want 'map', got string"},
			},
			"should return when tx is successfully sent": {
				params: map[string]interface{}{},
				result: util.Map{"hash": "0x0000"},
				mocker: func(tp *TestCase) {
					mockTxMod := mocks.NewMockTxModule(ctrl)
					mockTxMod.EXPECT().SendPayload(tp.params).Return(util.Map{
						"hash": "0x0000",
					})
					mods.Tx = mockTxMod
				},
			},
		}, api.sendPayload)
	})

	Describe(".getTransaction", func() {
		mods := &types.Modules{}
		api := &TransactionAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a string": {
				params:     map[string]interface{}{},
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a string", Data: ""},
			},
			"should return result on success": {
				params:     "0x123",
				statusCode: 200,
				result:     util.Map{"value": "10.2"},
				mocker: func(tc *TestCase) {
					mockTxModule := mocks.NewMockTxModule(ctrl)
					mockTxModule.EXPECT().Get("0x123").Return(util.Map{"value": "10.2"})
					mods.Tx = mockTxModule
				},
			},
		}, api.getTransaction)
	})
})
