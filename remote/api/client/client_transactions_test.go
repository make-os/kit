package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/imroc/req"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Describe(".SendTxPayload", func() {
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
				resp, err := client.SendTxPayload(map[string]interface{}{"type": 1})
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
				_, err := client.SendTxPayload(map[string]interface{}{"type": 1})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(`{"hash":"0x12345"}`))
			})
		})
	})
})
