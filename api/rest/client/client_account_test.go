package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/imroc/req"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller
	var client *Client

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = &Client{apiRoot: ""}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".AccountGetNonce", func() {
		When("address and block height are set", func() {
			It("should send `address` and `block height` in request and return nonce sent from server", func() {
				client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/user/get-nonce"))
					Expect(params).To(HaveLen(2))
					Expect(params).To(HaveKey("address"))
					Expect(params["address"]).To(Equal("addr1"))
					Expect(params).To(HaveKey("blockHeight"))
					Expect(params["blockHeight"]).To(Equal(uint64(100)))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{"nonce": "123"})
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				resp, err := client.AccountGetNonce("addr1", 100)
				Expect(err).To(BeNil())
				Expect(resp.Nonce).To(Equal("123"))
			})
		})
	})

	Describe(".AccountGet", func() {
		When("address and block height are set", func() {
			It("should send `address` and `block height` in request and return account sent from server", func() {
				client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/user/get-account"))
					Expect(params).To(HaveLen(2))
					Expect(params).To(HaveKey("address"))
					Expect(params["address"]).To(Equal("addr1"))
					Expect(params).To(HaveKey("blockHeight"))
					Expect(params["blockHeight"]).To(Equal(uint64(100)))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{
							"balance":             "979956",
							"delegatorCommission": "0.000000",
							"nonce":               "43",
						})
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				resp, err := client.AccountGet("addr1", 100)
				Expect(err).To(BeNil())
				Expect(resp.Balance).To(Equal(util.String("979956")))
				Expect(resp.Nonce).To(Equal(uint64(43)))
				Expect(resp.DelegatorCommission).To(Equal(float64(0)))
			})
		})
	})

	Describe(".AccountGetNextNonceUsingClients", func() {
		When("two clients are provided but the first one succeeds, the second should not be used", func() {
			It("should return the next nonce immediately after first client success", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().AccountGetNonce("addr1").
					Return(&types.AccountGetNonceResponse{Nonce: "10"}, nil).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().AccountGetNonce(gomock.Any()).Times(0)

				nextNonce, err := AccountGetNextNonceUsingClients([]RestClient{client, client2}, "addr1")
				Expect(err).To(BeNil())
				Expect(nextNonce).To(Equal("11"))
			})
		})

		When("two clients are provided but the first one fails, the second should be called", func() {
			It("should return the next nonce from the second client", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().AccountGetNonce("addr1").
					Return(nil, fmt.Errorf("error")).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().AccountGetNonce(gomock.Any()).
					Return(&types.AccountGetNonceResponse{Nonce: "11"}, nil).Times(1)

				nextNonce, err := AccountGetNextNonceUsingClients([]RestClient{client, client2}, "addr1")
				Expect(err).To(BeNil())
				Expect(nextNonce).To(Equal("12"))
			})
		})

		When("two clients are provided but both fail", func() {
			It("should return error", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().AccountGetNonce("addr1").
					Return(nil, fmt.Errorf("error")).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().AccountGetNonce(gomock.Any()).
					Return(nil, fmt.Errorf("error")).Times(1)

				nextNonce, err := AccountGetNextNonceUsingClients([]RestClient{client, client2}, "addr1")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("client[0]: error, client[1]: error"))
				Expect(nextNonce).To(Equal(""))
			})
		})
	})
})
