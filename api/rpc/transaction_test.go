package rpc

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Transaction", func() {
	var ctrl *gomock.Controller
	var txApi *TransactionAPI
	var mods *modules.Modules

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mods = &modules.Modules{}
		txApi = &TransactionAPI{mods}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".sendPayload()", func() {
		testCases := map[string]*TestCase{
			"when params is not a map[string]interface{}": {
				params: "{}",
				err:    &rpc.Err{Code: "60000", Message: "field:params, msg:wrong value type, want 'map', got string"},
			},
			"when tx is successfully sent": {
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
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := txApi.sendPayload(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})
})
