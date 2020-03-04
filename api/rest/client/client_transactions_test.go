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

	Describe(".TxSendPayload", func() {
		When("params of <map> type is set", func() {
			It("should send <map> in request and receive tx hash from server", func() {
				client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/tx/send-payload"))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{"hash": "0x12345"})
						w.WriteHeader(201)
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				resp, err := client.TxSendPayload(map[string]interface{}{"type": 1})
				Expect(err).To(BeNil())
				Expect(resp.Hash).To(Equal("0x12345"))
			})
		})

		When("status code is not 201", func() {
			It("should return response as error", func() {
				client.post = func(endpoint string, params map[string]interface{}) (resp *req.Resp, err error) {
					Expect(endpoint).To(Equal("/v1/tx/send-payload"))

					mockReqHandler := func(w http.ResponseWriter, r *http.Request) {
						data, _ := json.Marshal(util.Map{"hash": "0x12345"})
						w.WriteHeader(400)
						w.Write(data)
					}
					ts := httptest.NewServer(http.HandlerFunc(mockReqHandler))
					resp, _ = req.Get(ts.URL)

					return resp, nil
				}
				_, err := client.TxSendPayload(map[string]interface{}{"type": 1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(`{"hash":"0x12345"}`))
			})
		})
	})

	Describe(".TxSendPayloadUsingClients", func() {
		respData := &types.TxSendPayloadResponse{"0x123"}

		When("two clients are provided but the first one succeeds, the second should not be used", func() {
			It("should return the response after first client success", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().TxSendPayload(map[string]interface{}{}).Return(respData, nil).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().TxSendPayload(gomock.Any()).Times(0)

				resp, err := TxSendPayloadUsingClients([]RestClient{client, client2}, map[string]interface{}{})
				Expect(err).To(BeNil())
				Expect(resp.Hash).To(Equal(respData.Hash))
			})
		})

		When("two clients are provided but the first one fails, the second should be called", func() {
			It("should return the response from the second client succeeds", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().TxSendPayload(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().TxSendPayload(gomock.Any()).Return(respData, nil).Times(1)

				resp, err := TxSendPayloadUsingClients([]RestClient{client, client2}, map[string]interface{}{})
				Expect(err).To(BeNil())
				Expect(resp.Hash).To(Equal(respData.Hash))
			})
		})

		When("two clients are provided but both fail", func() {
			It("should return error", func() {
				client := mocks.NewMockRestClient(ctrl)
				client.EXPECT().TxSendPayload(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
				client2 := mocks.NewMockRestClient(ctrl)
				client2.EXPECT().TxSendPayload(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)

				_, err := TxSendPayloadUsingClients([]RestClient{client, client2}, map[string]interface{}{})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("client[0]: error, client[1]: error"))
			})
		})
	})
})
