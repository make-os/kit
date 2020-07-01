package api

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("RPC", func() {
	var ctrl *gomock.Controller
	var rpcApi *Manager

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		rpcApi = &Manager{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".echo()", func() {
		testCases := map[string]*TestCase{
			"should return params passed to it": {
				params: map[string]interface{}{"name": "major", "age": "1000"},
				result: util.Map{"data": map[string]interface{}{"name": "major", "age": "1000"}},
			},
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := rpcApi.echo(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})
})
