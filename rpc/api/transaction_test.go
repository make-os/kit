package api

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Transaction", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".sendPayload()", func() {
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
})
