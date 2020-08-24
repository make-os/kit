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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller
	var client *ClientV1
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = &ClientV1{apiRoot: ""}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetAccountNonce", func() {
		It("should send `address` and `block height` in request and return nonce sent from server", func() {
			client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/user/nonce"))
				Expect(params).To(HaveLen(2))
				Expect(params).To(HaveKey("address"))
				Expect(params["address"]).To(Equal("addr1"))
				Expect(params).To(HaveKey("height"))
				Expect(params["height"]).To(Equal(uint64(100)))

				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"nonce": "123"})
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)

				return resp, nil
			}
			resp, err := client.GetAccountNonce("addr1", 100)
			Expect(err).To(BeNil())
			Expect(resp.Nonce).To(Equal("123"))
		})
	})

	Describe(".GetAccount", func() {
		It("should send `address` and `block height` in request and return account sent from server", func() {
			client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/user/account"))
				Expect(params).To(HaveLen(2))
				Expect(params).To(HaveKey("address"))
				Expect(params["address"]).To(Equal("addr1"))
				Expect(params).To(HaveKey("height"))
				Expect(params["height"]).To(Equal(uint64(100)))

				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{
						"balance":             "979956",
						"delegatorCommission": 10,
						"nonce":               "43",
					})
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)

				return resp, nil
			}
			resp, err := client.GetAccount("addr1", 100)
			Expect(err).To(BeNil())
			Expect(resp.Balance).To(Equal(util.String("979956")))
			Expect(resp.Nonce.UInt64()).To(Equal(uint64(43)))
			Expect(resp.DelegatorCommission).To(Equal(float64(10)))
		})
	})

	Describe(".SendCoin()", func() {
		It("should return error if signing key is not set", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, nil
			}
			_, err := client.SendCoin(&types.SendCoinBody{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("signing key is required"))
		})

		It("should send payload and receive tx hash from server", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				Expect(endpoint).To(Equal("/v1/user/send"))
				Expect(params).To(And(
					HaveKey("sig"),
					HaveKey("timestamp"),
					HaveKey("senderPubKey"),
					HaveKey("value"),
					HaveKey("nonce"),
					HaveKey("to"),
					HaveKey("fee"),
					HaveKey("type"),
				))
				mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
					data, _ := json.Marshal(util.Map{"hash": "0x12345"})
					w.WriteHeader(201)
					w.Write(data)
				}
				ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
				resp, _ = req.Get(ts.URL)
				return resp, nil
			}
			resp, err := client.SendCoin(&types.SendCoinBody{
				Nonce:      1,
				Value:      100,
				Fee:        1,
				To:         key.Addr(),
				SigningKey: key,
			})
			Expect(err).To(BeNil())
			Expect(resp.Hash).To(Equal("0x12345"))
		})

		It("should return error if request fails", func() {
			client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
				return nil, fmt.Errorf("error")
			}
			_, err := client.SendCoin(&types.SendCoinBody{SigningKey: key})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})
})
