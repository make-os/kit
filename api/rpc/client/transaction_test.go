package client_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	client2 "github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/modules"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("Client", func() {
	var client *client2.RPCClient
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = client2.NewClient(&client2.Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".SendTxPayload", func() {
		It("should return ReqError when call failed", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return nil, 0, fmt.Errorf("error")
			})
			_, err := client.SendTxPayload(map[string]interface{}{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     client2.ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected result on success", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return util.Map{"hash": "0x123"}, 0, nil
			})
			txInfo, err := client.SendTxPayload(map[string]interface{}{})
			Expect(err).To(BeNil())
			Expect(txInfo.Hash).To(Equal("0x123"))
		})
	})

	Describe(".GetTransaction()", func() {
		It("should return ReqError when call failed", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("tx_get"))
				Expect(params).To(Equal("0x123"))
				return nil, 500, fmt.Errorf("error")
			})
			_, err := client.GetTransaction("0x123")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     client2.ErrCodeUnexpected,
				HttpCode: 500,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected repo object on success", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("tx_get"))
				return map[string]interface{}{
					"status": modules.TxStatusInMempool,
					"data":   map[string]interface{}{"value": "100.2"},
				}, 0, nil
			})
			res, err := client.GetTransaction("0x123")
			Expect(err).To(BeNil())
			Expect(res.Status).To(Equal(modules.TxStatusInMempool))
			Expect(res.Data).To(Equal(map[string]interface{}{"value": "100.2"}))
		})
	})
})
