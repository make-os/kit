package client

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/types/state"
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

	Describe(".AccountGet", func() {
		When("the RPC call returns an error", func() {
			It("should return the error wrapped in a StatusError", func() {
				client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
					return nil, 0, fmt.Errorf("bad thing happened")
				}
				_, err := client.AccountGet("addr", 100)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(&util.StatusError{
					Code:     "client_error",
					HttpCode: 0,
					Msg:      "bad thing happened",
					Field:    "",
				}))
			})
		})

		When("the response from the RPC call could not be decoded into the return object", func() {
			It("should return error", func() {
				client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
					return util.Map{"balance": 1000}, 0, nil
				}
				_, err := client.AccountGet("addr", 100)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(&util.StatusError{
					Code:     "client_error",
					HttpCode: 0,
					Msg:      "failed to decode call response: field:balance, msg:invalid value type: has int, wants string",
					Field:    "",
				}))
			})
		})

		When("the RPC call returns a result", func() {
			It("should return the result", func() {
				client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
					return util.Map{"balance": "1000"}, 0, nil
				}
				acct, err := client.AccountGet("addr", 100)
				Expect(err).To(BeNil())
				Expect(acct.Balance).To(Equal(util.String("1000")))
			})
		})
	})

	Describe(".GetNextNonceOfAccountUsingRPCClient", func() {
		When("client call returns error", func() {
			It("should return error", func() {
				mockClient := NewMockClient(ctrl)
				mockClient.EXPECT().AccountGet("addr1").Return(nil, &util.StatusError{Msg: "bad thing"})
				_, err := GetNextNonceOfAccountUsingRPCClient("addr1", mockClient)
				Expect(err).NotTo(BeNil())
				Expect(err.Msg).To(Equal("bad thing"))
			})
		})

		When("client call a nonce", func() {
			It("should be incremented by 1 and returned", func() {
				mockClient := NewMockClient(ctrl)
				mockClient.EXPECT().AccountGet("addr1").Return(&state.Account{Nonce: 10}, nil)
				nonce, err := GetNextNonceOfAccountUsingRPCClient("addr1", mockClient)
				Expect(err).To(BeNil())
				Expect(nonce).To(Equal("11"))
			})
		})
	})
})
