package client

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RPC", func() {
	var client *RPCClient
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetMethods", func() {
		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("rpc_methods"))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.RPC().GetMethods()
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return when unable to decode call result", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("rpc_methods"))
				return map[string]interface{}{"methods": 100}, 0, nil
			}
			_, err := client.RPC().GetMethods()
			Expect(err).ToNot(BeNil())
			Expect(err.(*util.ReqError).Code).To(Equal(ErrCodeDecodeFailed))
		})

		It("should return nil on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("rpc_methods"))
				return map[string]interface{}{"methods": []map[string]interface{}{
					{"name": "get"},
				}}, 0, nil
			}
			resp, err := client.RPC().GetMethods()
			Expect(err).To(BeNil())
			Expect(resp.Methods).To(HaveLen(1))
			Expect(resp.Methods[0].Name).To(Equal("get"))
		})
	})
})
