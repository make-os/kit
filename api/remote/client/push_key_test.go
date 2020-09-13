package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/imroc/req"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller
	var client *RemoteClient
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		client = &RemoteClient{apiRoot: ""}
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetOwnerNonce", func() {
		It("should send key id and block height in request and receive nonce from server", func() {
			client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/pk/owner-nonce"))
				Expect(params).To(HaveLen(2))
				Expect(params).To(HaveKey("id"))
				Expect(params["id"]).To(Equal("addr1"))
				Expect(params["height"]).To(Equal(uint64(100)))

				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"nonce": "123"})
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)

				return resp, nil
			}
			resp, err := client.PushKey().GetOwnerNonce("addr1", 100)
			Expect(err).To(BeNil())
			Expect(resp.Nonce).To(Equal("123"))
		})
	})

	Describe(".Get", func() {
		It("should send keys id and block height in request and receive nonce from server", func() {
			expectedPubKey, _ := crypto.PubKeyFromBase58("49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ")
			client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/pk/find"))
				Expect(params).To(HaveLen(2))
				Expect(params).To(HaveKey("id"))
				Expect(params["id"]).To(Equal("pushKeyID"))
				Expect(params["height"]).To(Equal(uint64(100)))

				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"address": "addr1", "pubKey": expectedPubKey.ToPublicKey()})
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)

				return resp, nil
			}
			resp, err := client.PushKey().Get("pushKeyID", 100)
			Expect(err).To(BeNil())
			Expect(resp.Address).To(Equal(identifier.Address("addr1")))
			Expect(resp.PubKey).To(Equal(expectedPubKey.ToPublicKey()))
		})
	})

	Describe(".Register", func() {
		It("should return error if signing key is not set", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, nil
			}
			_, err := client.PushKey().Register(&types.BodyRegisterPushKey{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("signing key is required"))
		})

		It("should send payload and receive address tx hash from server", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/pk/register"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("feeCap"),
					HaveKey("pubKey"),
					HaveKey("nonce"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"address": "pk1abc", "hash": "0x12345"})
					w.WriteHeader(201)
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)
				return resp, nil
			}
			resp, err := client.PushKey().Register(&types.BodyRegisterPushKey{
				Nonce:      1,
				Fee:        1,
				Scopes:     []string{"ns/repo", "repo1"},
				FeeCap:     10.5,
				PublicKey:  key.PubKey().ToPublicKey(),
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Address).To(Equal("pk1abc"))
			Expect(resp.Hash).To(Equal("0x12345"))
		})

		It("should return error if request fails", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, fmt.Errorf("error")
			}
			_, err := client.PushKey().Register(&types.BodyRegisterPushKey{SigningKey: key})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})
})
