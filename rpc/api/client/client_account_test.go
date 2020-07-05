package client_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	client2 "gitlab.com/makeos/mosdef/rpc/api/client"
	"gitlab.com/makeos/mosdef/util"
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

	Describe(".GetAccount", func() {
		It("should return StatusError when RPC call returns an error", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("user_get"))
				return nil, 0, fmt.Errorf("error")
			})
			_, err := client.GetAccount("addr", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     client2.ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return error when response could not be decoded", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("user_get"))
				return util.Map{"balance": 1000}, 0, nil
			})
			_, err := client.GetAccount("addr", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     "decode_error",
				HttpCode: 500,
				Msg:      "field:balance, msg:invalid value type: has int, wants string",
				Field:    "",
			}))
		})

		It("should return expected result on success", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("user_get"))
				return util.Map{"balance": "1000"}, 0, nil
			})
			acct, err := client.GetAccount("addr", 100)
			Expect(err).To(BeNil())
			Expect(acct.Balance).To(Equal(util.String("1000")))
		})
	})
})
