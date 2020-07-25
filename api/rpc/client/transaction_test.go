package client

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/lobe/util"
)

var _ = Describe("Client", func() {
	var client *RPCClient
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".SendTxPayload", func() {
		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.SendTxPayload(map[string]interface{}{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected result on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return util.Map{"hash": "0x123"}, 0, nil
			}
			txInfo, err := client.SendTxPayload(map[string]interface{}{})
			Expect(err).To(BeNil())
			Expect(txInfo.Hash).To(Equal("0x123"))
		})
	})

	Describe(".GetTransaction()", func() {
		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("tx_get"))
				Expect(params).To(Equal("0x123"))
				return nil, 500, fmt.Errorf("error")
			}
			_, err := client.GetTransaction("0x123")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 500,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected repo object on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("tx_get"))
				return util.Map{"value": "100.2"}, 0, nil
			}
			res, err := client.GetTransaction("0x123")
			Expect(err).To(BeNil())
			Expect(res["value"]).To(Equal("100.2"))
		})
	})
})
