package client

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/crypto/ed25519"
	types2 "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var _ = Describe("Client", func() {

	Describe(".NewClient", func() {
		When("host and port are set", func() {
			It("should not panic", func() {
				Expect(func() { NewClient(&types.Options{Host: "127.0.0.1", Port: 5000}) }).ToNot(Panic())
			})
		})
	})

	Describe(".Call", func() {
		It("should return error when options haven't been set", func() {
			c := RPCClient{opts: &types.Options{Host: "127.0.0.1"}}
			_, _, err := c.Call("", nil)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("http client and options not set"))
		})
	})

	Describe(".GetOptions", func() {
		It("should return options", func() {
			opts := &types.Options{Host: "hostA", Port: 9000}
			Expect(NewClient(opts).GetOptions()).To(Equal(opts))
		})
	})

	Describe(".makeClientStatusErr", func() {
		It("should return a ReqErr that describes a client error", func() {
			err := makeClientStatusErr("something bad on client: code %d", 11)
			Expect(err.Field).To(Equal(""))
			Expect(err.Code).To(Equal("client_error"))
			Expect(err.HttpCode).To(Equal(0))
			Expect(err.Msg).To(Equal("something bad on client: code 11"))
		})
	})

	Describe(".makeStatusErrorFromCallErr", func() {
		When("error does not contain a json object string", func() {
			It("should create unexpected_error", func() {
				err := makeStatusErrorFromCallErr(500, fmt.Errorf("some bad error"))
				Expect(err.HttpCode).To(Equal(500))
				Expect(err.Msg).To(Equal("some bad error"))
				Expect(err.Code).To(Equal(ErrCodeUnexpected))
				Expect(err.Field).To(Equal(""))
			})
		})

		When("error contains a status error in string format", func() {
			It("should format the string and return a ReqError object", func() {
				se := errors.ReqErr(500, "some_error", "field_a", "msg")
				err := makeStatusErrorFromCallErr(500, fmt.Errorf(se.Error()))
				Expect(err.HttpCode).To(Equal(500))
				Expect(err.Msg).To(Equal("msg"))
				Expect(err.Code).To(Equal("some_error"))
				Expect(err.Field).To(Equal("field_a"))
			})
		})

		When("status code is not 0 and error is json encoding of rpc.Response", func() {
			It("should return ReqErr populated with values from the encoded rpc.Response", func() {
				err := rpc.Response{Err: &rpc.Err{Code: "bad_code", Message: "we have a problem", Data: "bad_field"}}
				se := makeStatusErrorFromCallErr(500, fmt.Errorf(`%s`, err.ToJSON()))
				Expect(se.Code).To(Equal("bad_code"))
				Expect(se.HttpCode).To(Equal(500))
				Expect(se.Msg).To(Equal("we have a problem"))
				Expect(se.Field).To(Equal("bad_field"))
			})
		})
	})
})

var _ = Describe("PushKeyAPI", func() {
	var client *RPCClient
	var ctrl *gomock.Controller
	var key = ed25519.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&types.Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetOwner", func() {
		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("pk_getOwner"))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.PushKey().GetOwner("pk1_abc", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
			_, err := client.PushKey().GetOwner("pk1_abc", 100)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
			acct, err := client.PushKey().GetOwner("pk1_abc", 100)
			Expect(err).To(BeNil())
			Expect(acct.Balance).To(Equal(util.String("1000")))
		})
	})

	Describe(".Register()", func() {
		It("should return ReqError when signing key is not provided", func() {
			_, err := client.PushKey().Register(&api.BodyRegisterPushKey{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
					HaveKey("feeCap"),
					HaveKey("pubKey"),
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("type"),
					HaveKey("scopes"),
					HaveKey("nonce"),
					HaveKey("fee"),
				))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.PushKey().Register(&api.BodyRegisterPushKey{
				Nonce:      100,
				Fee:        1,
				Scopes:     []string{"scope1"},
				FeeCap:     1.2,
				PublicKey:  key.PubKey().ToPublicKey(),
				SigningKey: key,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected address and transaction hash on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("pk_register"))
				return util.Map{"address": "pk1abc", "hash": "0x123"}, 0, nil
			}
			resp, err := client.PushKey().Register(&api.BodyRegisterPushKey{
				Nonce:      100,
				Fee:        1,
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Address).To(Equal("pk1abc"))
			Expect(resp.Hash).To(Equal("0x123"))
		})
	})
})

var _ = Describe("RepoAPI", func() {
	var client *RPCClient
	var ctrl *gomock.Controller
	var key = ed25519.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&types.Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".Create", func() {
		It("should return ReqError when signing key is not provided", func() {
			_, err := client.Repo().Create(&api.BodyCreateRepo{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
			_, err := client.Repo().Create(&api.BodyCreateRepo{
				Name:       "repo1",
				Nonce:      100,
				Value:      10,
				Fee:        1,
				Config:     state.DefaultRepoConfig.ToBasicMap(),
				SigningKey: key,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
			resp, err := client.Repo().Create(&api.BodyCreateRepo{
				Name:       "repo1",
				Nonce:      100,
				Value:      10,
				Fee:        1,
				Config:     state.DefaultRepoConfig.ToBasicMap(),
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Address).To(Equal("r/repo1"))
			Expect(resp.Hash).To(Equal("0x123"))
		})
	})

	Describe(".Get", func() {
		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_get"))
				Expect(params).To(And(
					HaveKey("name"),
					HaveKey("height"),
					HaveKey("noProposals"),
				))
				return nil, 500, fmt.Errorf("error")
			}
			_, err := client.Repo().Get("repo1", &api.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 500,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return ReqError when unable to decode call result", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_get"))
				return util.Map{"balance": struct{}{}}, 0, nil
			}
			_, err := client.Repo().Get("repo1", &api.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).ToNot(BeNil())
			Expect(err.(*errors.ReqError).Code).To(Equal(ErrCodeDecodeFailed))
			Expect(err.(*errors.ReqError).HttpCode).To(Equal(500))
			Expect(err.(*errors.ReqError).Msg).To(ContainSubstring("expected type 'util.String', got unconvertible type 'struct {}'"))
		})

		It("should return expected repo object on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_get"))
				return util.Map{"balance": "100.2"}, 0, nil
			}
			res, err := client.Repo().Get("repo1", &api.GetRepoOpts{Height: 100, NoProposals: true})
			Expect(err).To(BeNil())
			Expect(res.Balance.String()).To(Equal("100.2"))
		})
	})

	Describe(".AddContributors", func() {
		It("should return ReqError when signing key is not provided", func() {
			_, err := client.Repo().AddContributors(&api.BodyAddRepoContribs{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeBadParam,
				HttpCode: 400,
				Msg:      "signing key is required",
				Field:    "signingKey",
			}))
		})

		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_addContributor"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("value"),
					HaveKey("id"),
					HaveKey("name"),
					HaveKey("namespace"),
					HaveKey("namespaceOnly"),
					HaveKey("policies"),
					HaveKey("feeCap"),
					HaveKey("feeMode"),
					HaveKey("nonce"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.Repo().AddContributors(&api.BodyAddRepoContribs{
				RepoName:      "repo1",
				ProposalID:    "1",
				PushKeys:      []string{"pk1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz"},
				FeeCap:        13.2,
				FeeMode:       12,
				Nonce:         1,
				Value:         10,
				Fee:           1.2,
				Namespace:     "ns1",
				NamespaceOnly: "ns1",
				Policies:      []*state.ContributorPolicy{{Object: "obj1", Action: "act1"}},
				SigningKey:    key,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected repo object on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return util.Map{"address": "r/repo1", "hash": "0x123"}, 0, nil
			}
			resp, err := client.Repo().AddContributors(&api.BodyAddRepoContribs{SigningKey: key})
			Expect(err).To(BeNil())
			Expect(resp.Hash).To(Equal("0x123"))
		})
	})

	Describe(".VoteProposal", func() {
		It("should return ReqError when signing key is not provided", func() {
			_, err := client.Repo().VoteProposal(&api.BodyRepoVote{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeBadParam,
				HttpCode: 400,
				Msg:      "signing key is required",
				Field:    "signingKey",
			}))
		})

		It("should return ReqError when call failed", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("repo_vote"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("id"),
					HaveKey("vote"),
					HaveKey("nonce"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				return nil, 0, fmt.Errorf("error")
			}
			_, err := client.Repo().VoteProposal(&api.BodyRepoVote{
				RepoName:   "repo1",
				ProposalID: "1",
				Nonce:      1,
				Fee:        1.2,
				SigningKey: key,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected repo object on success", func() {
			client.call = func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return util.Map{"hash": "0x123"}, 0, nil
			}
			resp, err := client.Repo().VoteProposal(&api.BodyRepoVote{SigningKey: key})
			Expect(err).To(BeNil())
			Expect(resp.Hash).To(Equal("0x123"))
		})
	})
})

var _ = Describe("RPCAPI", func() {
	var client *RPCClient
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&types.Options{Host: "127.0.0.1", Port: 8000})
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
			Expect(err).To(Equal(&errors.ReqError{
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
			Expect(err.(*errors.ReqError).Code).To(Equal(ErrCodeDecodeFailed))
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
			Expect(resp).To(HaveLen(1))
			Expect(resp[0].Name).To(Equal("get"))
		})
	})
})

var _ = Describe("TxAPI", func() {
	var client *RPCClient
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&types.Options{Host: "127.0.0.1", Port: 8000})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".Send", func() {
		It("should return ReqError when call failed", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return nil, 0, fmt.Errorf("error")
			})
			_, err := client.Tx().Send(map[string]interface{}{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 0,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected result on success", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				return util.Map{"hash": "0x123"}, 0, nil
			})
			txInfo, err := client.Tx().Send(map[string]interface{}{})
			Expect(err).To(BeNil())
			Expect(txInfo.Hash).To(Equal("0x123"))
		})
	})

	Describe(".Get()", func() {
		It("should return ReqError when call failed", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("tx_get"))
				Expect(params).To(Equal("0x123"))
				return nil, 500, fmt.Errorf("error")
			})
			_, err := client.Tx().Get("0x123")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
				Code:     ErrCodeUnexpected,
				HttpCode: 500,
				Msg:      "error",
				Field:    "",
			}))
		})

		It("should return expected repo object on success", func() {
			client.SetCallFunc(func(method string, params interface{}) (res util.Map, statusCode int, err error) {
				Expect(method).To(Equal("tx_get"))
				return map[string]interface{}{
					"status": types2.TxStatusInMempool,
					"data":   map[string]interface{}{"value": "100.2"},
				}, 0, nil
			})
			res, err := client.Tx().Get("0x123")
			Expect(err).To(BeNil())
			Expect(res.Status).To(Equal(types2.TxStatusInMempool))
			Expect(res.Data).To(Equal(map[string]interface{}{"value": "100.2"}))
		})
	})
})

var _ = Describe("UserAPI", func() {
	var client *RPCClient
	var ctrl *gomock.Controller
	var key = ed25519.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = NewClient(&types.Options{Host: "127.0.0.1", Port: 8000})
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
			Expect(err).To(Equal(&errors.ReqError{
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
			Expect(err).To(Equal(&errors.ReqError{
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
			_, err := client.User().Send(&api.BodySendCoin{
				SigningKey: nil,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
			_, err := client.User().Send(&api.BodySendCoin{
				Nonce:      100,
				Value:      10,
				Fee:        1,
				To:         key.Addr(),
				SigningKey: key,
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(&errors.ReqError{
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
			resp, err := client.User().Send(&api.BodySendCoin{
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
