package client

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/util"
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
		It("should return StatusError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.SendTxPayload(map[string]interface{}{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
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
})
