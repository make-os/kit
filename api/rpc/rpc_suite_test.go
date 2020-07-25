package rpc

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/lobe/rpc"
)

type TestCase struct {
	params     interface{}
	body       string
	statusCode int
	mocker     func(tc *TestCase)
	result     map[string]interface{}
	err        *rpc.Err
}

func testCases(testCases map[string]*TestCase, f func(params interface{}) *rpc.Response) {
	for _tc, _tp := range testCases {
		tc, tp := _tc, _tp
		It(tc, func() {
			if tp.mocker != nil {
				tp.mocker(tp)
			}
			resp := f(tp.params)
			Expect(resp).To(Equal(&rpc.Response{
				JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
			}))
		})
	}
}

func TestRpc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rpc Suite")
}
