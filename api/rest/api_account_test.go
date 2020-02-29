package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	modules2 "gitlab.com/makeos/mosdef/types/modules"
)

var _ = Describe("Account", func() {
	var ctrl *gomock.Controller
	var mockModuleHub *mocks.MockModuleHub
	var restApi *RESTApi

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockModuleHub = mocks.NewMockModuleHub(ctrl)
		restApi = &RESTApi{
			mods: mockModuleHub,
			log:  logger.NewLogrusNoOp(),
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetAccountNonce", func() {
		var w *httptest.ResponseRecorder
		var req *http.Request
		var testCases = map[string]TestCase{
			"address is not provided": {
				params:     nil,
				body:       `{"error":{"code":"60000","field":"address","msg":"address is required"}}`,
				statusCode: 400,
			},
			"blockHeight type is a valid integer": {
				params:     map[string]string{"address": "addr", "blockHeight": "1ab"},
				body:       `{"error":{"code":"60000","field":"blockHeight","msg":"blockHeight requires a numeric value"}}`,
				statusCode: 400,
			},
			"address should be passed to AccountModule#GetNonce": {
				params:     map[string]string{"address": "maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc"},
				body:       `{"nonce":"100"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockAcctModule := mocks.NewMockAccountModule(ctrl)
					mockAcctModule.EXPECT().GetNonce(
						"maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc", uint64(0)).Return("100")
					mockModuleHub.EXPECT().GetModules().Return(&modules2.Modules{Account: mockAcctModule})
				},
			},
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			When(tc, func() {
				It(fmt.Sprintf("should return statusCode=%d, msg=%s", tp.statusCode, tp.body), func() {
					w = httptest.NewRecorder()
					req = httptest.NewRequest("GET", "http://", nil)
					q := req.URL.Query()
					for k, v := range tp.params {
						q.Add(k, v)
					}

					if tp.mocker != nil {
						tp.mocker(&tp)
					}

					req.URL.RawQuery = q.Encode()
					restApi.GetAccountNonce(w, req)
					_ = req.Body.Close()
					Expect(w.Code).To(Equal(tp.statusCode))
					Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
				})
			})
		}
	})

	Describe(".GetAccountNonce", func() {
		var w *httptest.ResponseRecorder
		var req *http.Request
		var testCases = map[string]TestCase{
			"address is not provided": {
				params:     nil,
				body:       `{"error":{"code":"60000","field":"address","msg":"address is required"}}`,
				statusCode: 400,
			},
			"blockHeight type is a valid integer": {
				params:     map[string]string{"address": "addr", "blockHeight": "1ab"},
				body:       `{"error":{"code":"60000","field":"blockHeight","msg":"blockHeight requires a numeric value"}}`,
				statusCode: 400,
			},
			"address is valid": {
				params:     map[string]string{"address": "maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc"},
				body:       `{"nonce":"100"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockAcctModule := mocks.NewMockAccountModule(ctrl)
					mockAcctModule.EXPECT().GetNonce(
						"maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc", uint64(0)).Return("100")
					mockModuleHub.EXPECT().GetModules().Return(&modules2.Modules{Account: mockAcctModule})
				},
			},
		}

		for _tc, _tp := range testCases {
			tc, tp := _tc, _tp
			When(tc, func() {
				It(fmt.Sprintf("should return statusCode=%d, msg=%s", tp.statusCode, tp.body), func() {
					w = httptest.NewRecorder()
					req = httptest.NewRequest("GET", "http://", nil)
					q := req.URL.Query()
					for k, v := range tp.params {
						q.Add(k, v)
					}

					if tp.mocker != nil {
						tp.mocker(&tp)
					}

					req.URL.RawQuery = q.Encode()
					restApi.GetAccountNonce(w, req)
					_ = req.Body.Close()
					Expect(w.Code).To(Equal(tp.statusCode))
					Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
				})
			})
		}
	})
})
