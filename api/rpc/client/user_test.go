package client

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var client *RPCClient
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".Get", func() {
		It("should return ReqError when RPC call returns an error", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("user_get"))
				return nil, 0, fmt.Errorf("error")
			})
			_, err := client.User().Get("addr", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     ErrCodeUnexpected,
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
			_, err := client.User().Get("addr", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
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
			acct, err := client.User().Get("addr", 100)
			Expect(err).To(BeNil())
			Expect(acct.Balance).To(Equal(util.String("1000")))
		})
	})

	Describe(".Send()", func() {
		It("should return ReqError when signing key is not provided", func() {
			_, err := client.User().Send(&types.SendCoinBody{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.ReqError{
				Code:     ErrCodeBadParam,
				HttpCode: 400,
				Msg:      "signing key is required",
				Field:    "signingKey",
			}))
		})

		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(params).To(And(
					HaveKey("senderPubKey"),
					HaveKey("value"),
					HaveKey("to"),
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("type"),
					HaveKey("nonce"),
					HaveKey("fee"),
				))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.User().Send(&types.SendCoinBody{
				Nonce:      100,
				Value:      10,
				Fee:        1,
				To:         key.Addr(),
				SigningKey: key,
			})
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
				Expect(method).To(Equal("user_send"))
				return util.Map{"hash": "0x123"}, 0, nil
			}
			resp, err := client.User().Send(&types.SendCoinBody{
				Nonce:      100,
				Value:      10,
				Fee:        1,
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Hash).To(Equal("0x123"))
		})
	})
})
