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
		When("the RPC call returns an error", func() {
			It("should return the error wrapped in a StatusError", func() {
				client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
					return nil, 0, fmt.Errorf("bad thing happened")
				})
				_, err := client.GetAccount("addr", 100)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(&util.StatusError{
					Code:     client2.ErrCodeUnexpected,
					HttpCode: 0,
					Msg:      "bad thing happened",
					Field:    "",
				}))
			})
		})

		When("the response from the RPC call could not be decoded into the return object", func() {
			It("should return error", func() {
				client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
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
		})

		When("the RPC call returns a result", func() {
			It("should return the result", func() {
				client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
					return util.Map{"balance": "1000"}, 0, nil
				})
				acct, err := client.GetAccount("addr", 100)
				Expect(err).To(BeNil())
				Expect(acct.Balance).To(Equal(util.String("1000")))
			})
		})
	})
})
