package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Tx", func() {
	var ctrl *gomock.Controller
	var mockModuleHub *mocks.MockModulesHub
	var restApi *API

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockModuleHub = mocks.NewMockModulesHub(ctrl)
		restApi = &API{
			mods: mockModuleHub,
			log:  logger.NewLogrusNoOp(),
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".SendTxPayload", func() {
		var w *httptest.ResponseRecorder
		var req *http.Request
		var testCases = map[string]TestCase{
			"body should be passed to TxModule#SendPayload": {
				params:     map[string]string{},
				body:       `{"hash":"0x000000"}`,
				statusCode: 201,
				mocker: func(tc *TestCase) {
					mockTxModule := mocks.NewMockTxModule(ctrl)
					mockTxModule.EXPECT().
						SendPayload(make(map[string]interface{})).
						Return(util.Map{"hash": "0x000000"})
					mockModuleHub.EXPECT().GetModules().Return(&types.Modules{Tx: mockTxModule})
				},
			},
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			When(tc, func() {
				It(fmt.Sprintf("should return statusCode=%d, msg=%s", tp.statusCode, tp.body), func() {
					w = httptest.NewRecorder()
					body, _ := json.Marshal(tp.params)
					req = httptest.NewRequest("POST", "http://", bytes.NewReader(body))
					if tp.mocker != nil {
						tp.mocker(&tp)
					}

					restApi.SendTxPayload(w, req)
					_ = req.Body.Close()
					Expect(w.Code).To(Equal(tp.statusCode))
					Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
				})
			})
		}
	})
})
