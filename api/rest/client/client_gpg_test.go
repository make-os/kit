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
		client = &Client{apiRoot: ""}
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GPGGetNonceOfOwner", func() {
		When("gpg id and block height are set", func() {
			It("should send gpg id and block height in request and receive nonce from server", func() {
				client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/gpg/owner-nonce"))
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
				resp, err := client.GPGGetNonceOfOwner("addr1", 100)
				Expect(err).To(BeNil())
				Expect(resp.Nonce).To(Equal("123"))
			})
		})
	})

	Describe(".GPGFind", func() {
		When("gpg id and block height are set", func() {
			It("should send gpg id and block height in request and receive nonce from server", func() {
				client.get = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/gpg/find"))
					Expect(params).To(HaveLen(2))
					Expect(params).To(HaveKey("id"))
					Expect(params["id"]).To(Equal("gpgID"))
					Expect(params["blockHeight"]).To(Equal(uint64(100)))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{"address": "addr1", "pubKey": "-----BEGIN PGP PUBLIC KEY BLOCK..."})
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				resp, err := client.GPGFind("gpgID", 100)
				Expect(err).To(BeNil())
				Expect(resp.Address).To(Equal(util.String("addr1")))
				Expect(resp.PubKey).To(Equal("-----BEGIN PGP PUBLIC KEY BLOCK..."))
			})
		})
	})

	Describe(".GPGGetNextNonceOfOwnerUsingClients", func() {
		When("two clients are provided but the first one succeeds, the second should not be used", func() {
			It("should return the next nonce immediately after first client success", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().GPGGetNonceOfOwner("gpgID").Return(&types.AccountGetNonceResponse{Nonce: "10"}, nil).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().GPGGetNonceOfOwner(gomock.Any()).Times(0)

				nextNonce, err := GPGGetNextNonceOfOwnerUsingClients([]RestClient{client, client2}, "gpgID")
				Expect(err).To(BeNil())
				Expect(nextNonce).To(Equal("11"))
			})
		})

		When("two clients are provided but the first one fails, the second should be called", func() {
			It("should return the next nonce from the second client", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().GPGGetNonceOfOwner("addr1").Return(nil, fmt.Errorf("error")).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().GPGGetNonceOfOwner(gomock.Any()).Return(&types.AccountGetNonceResponse{Nonce: "11"}, nil).Times(1)

				nextNonce, err := GPGGetNextNonceOfOwnerUsingClients([]RestClient{client, client2}, "addr1")
				Expect(err).To(BeNil())
				Expect(nextNonce).To(Equal("12"))
			})
		})

		When("two clients are provided but both fail", func() {
			It("should return error", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().GPGGetNonceOfOwner("addr1").Return(nil, fmt.Errorf("error")).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().GPGGetNonceOfOwner(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)

				nextNonce, err := GPGGetNextNonceOfOwnerUsingClients([]RestClient{client, client2}, "addr1")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("client[0]: error, client[1]: error"))
				Expect(nextNonce).To(Equal(""))
			})
		})
	})
})
