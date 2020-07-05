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

	Describe(".GetPushKeyOwner", func() {
		It("should return StatusError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("pk_getOwner"))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.GetPushKeyOwner("push1_abc", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return error when RPC call response could not be decoded", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("pk_getOwner"))
				return util.Map{"balance": 1000}, 0, nil
			}
			_, err := client.GetPushKeyOwner("push1_abc", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     "decode_error",
				HttpCode: 500,
				Msg:      "field:balance, msg:invalid value type: has int, wants string",
				Field:    "",
			}))
		})

		It("should return expected result on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("pk_getOwner"))
				return util.Map{"balance": "1000"}, 0, nil
			}
			acct, err := client.GetPushKeyOwner("push1_abc", 100)
			Expect(err).To(BeNil())
			Expect(acct.Balance).To(Equal(util.String("1000")))
		})
	})
})
