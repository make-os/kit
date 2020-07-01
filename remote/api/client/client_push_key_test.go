package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/imroc/req"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller
	var client *ClientV1

	BeforeEach(func() {
		client = &ClientV1{apiRoot: ""}
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetPushKeyOwnerNonce", func() {
		When("keys id and block height are set", func() {
			It("should send keys id and block height in request and receive nonce from server", func() {
				client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/pk/owner-nonce"))
					Expect(params).To(HaveLen(2))
					Expect(params).To(HaveKey("id"))
					Expect(params["id"]).To(Equal("addr1"))
					Expect(params["blockHeight"]).To(Equal(uint64(100)))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{"nonce": "123"})
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				resp, err := client.GetPushKeyOwnerNonce("addr1", 100)
				Expect(err).To(BeNil())
				Expect(resp.Nonce).To(Equal("123"))
			})
		})
	})

	Describe(".GetPushKey", func() {
		When("keys id and block height are set", func() {
			It("should send keys id and block height in request and receive nonce from server", func() {
				client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/pk/find"))
					Expect(params).To(HaveLen(2))
					Expect(params).To(HaveKey("id"))
					Expect(params["id"]).To(Equal("pushKeyID"))
					Expect(params["blockHeight"]).To(Equal(uint64(100)))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{"address": "addr1", "pubKey": "49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ"})
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				resp, err := client.GetPushKey("pushKeyID", 100)
				Expect(err).To(BeNil())
				Expect(resp.Address).To(Equal(util.Address("addr1")))
				expectedPubKey, _ := crypto.PubKeyFromBase58("49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ")
				Expect(resp.PubKey).To(Equal(expectedPubKey.ToPublicKey()))
			})
		})
	})
})
