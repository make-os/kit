package client

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
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

	Describe(".CreateRepo", func() {
		It("should return StatusError when signing key is not provided", func() {
			_, err := client.CreateRepo(&api.CreateRepoBody{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     ErrCodeBadParam,
				HttpCode: 400,
				Msg:      "signing key is required",
				Field:    "signingKey",
			}))
		})

		It("should return StatusError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(params).To(And(
					HaveKey("senderPubKey"),
					HaveKey("value"),
					HaveKey("config"),
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("type"),
					HaveKey("name"),
					HaveKey("nonce"),
					HaveKey("fee"),
				))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.CreateRepo(&api.CreateRepoBody{
				Name:       "repo1",
				Nonce:      100,
				Value:      "10",
				Fee:        "1",
				Config:     state.DefaultRepoConfig,
				SigningKey: key,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected repo object on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_create"))
				return util.Map{"address": "r/repo1", "hash": "0x123"}, 0, nil
			}
			resp, err := client.CreateRepo(&api.CreateRepoBody{
				Name:       "repo1",
				Nonce:      100,
				Value:      "10",
				Fee:        "1",
				Config:     state.DefaultRepoConfig,
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Address).To(Equal("r/repo1"))
			Expect(resp.Hash).To(Equal("0x123"))
		})
	})

	Describe(".GetRepo", func() {
		It("should return StatusError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_get"))
				Expect(params).To(And(
					HaveKey("name"),
					HaveKey("height"),
					HaveKey("noProposals"),
				))
				return nil, 500, fmt.Errorf("error")
			}
			_, err := client.GetRepo("repo1", &api.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&util.StatusError{
				Code:     ErrCodeUnexpected,
				HttpCode: 500,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return StatusError when unable to decode call result", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_get"))
				return util.Map{"balance": struct{}{}}, 0, nil
			}
			_, err := client.GetRepo("repo1", &api.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).ToNot(BeNil())
			Expect(err.Code).To(Equal(ErrCodeDecodeFailed))
			Expect(err.HttpCode).To(Equal(500))
			Expect(err.Msg).To(ContainSubstring("expected type 'util.String', got unconvertible type 'struct {}'"))
		})

		It("should return expected repo object on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_get"))
				return util.Map{"balance": "100.2"}, 0, nil
			}
			res, err := client.GetRepo("repo1", &api.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).To(BeNil())
			Expect(res.Balance.String()).To(Equal("100.2"))
		})
	})
})
