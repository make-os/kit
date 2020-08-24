package rpc

import (
	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("RPC", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".echo()", func() {
		api := &Manager{}
		testCases(map[string]*TestCase{
			"should return params passed to it": {
				params: map[string]interface{}{"name": "major", "age": "1000"},
				result: util.Map{"data": map[string]interface{}{"name": "major", "age": "1000"}},
			},
		}, api.echo)
	})
})
