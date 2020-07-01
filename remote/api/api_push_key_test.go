package api

import (
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

var _ = Describe("GPG", func() {
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

	Describe(".GetPushKey", func() {
		var w *httptest.ResponseRecorder
		var req *http.Request
		var testCases = map[string]TestCase{
			"id and blockHeight should be passed to PushKeyModule#Get": {
				params:     map[string]string{"id": "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", "blockHeight": "1"},
				body:       `{"address":"maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc","pubKey":"49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().
						Get("push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", uint64(1)).
						Return(util.Map{
							"pubKey":  "49G1iGk8fY7RQcJQ7LfQdThdyfaN8dKfxhGQSh8uuNaK35CgazZ",
							"address": "maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc"})
					mockModuleHub.EXPECT().GetModules().Return(&types.Modules{PushKey: mockPushKeyModule})
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
					restApi.GetPushKey(w, req)
					_ = req.Body.Close()
					Expect(w.Code).To(Equal(tp.statusCode))
					Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
				})
			})
		}
	})

	Describe(".GetPushKeyOwnerNonce", func() {
		var w *httptest.ResponseRecorder
		var req *http.Request
		var testCases = map[string]TestCase{
			"id and blockHeight should be passed to PushKeyModule#GetAccountOfOwner": {
				params:     map[string]string{"id": "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", "blockHeight": "1"},
				body:       `{"nonce":"1000"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockPushKeyModule := mocks.NewMockPushKeyModule(ctrl)
					mockPushKeyModule.EXPECT().
						GetAccountOfOwner("push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", uint64(1)).
						Return(util.Map{"nonce": "1000"})
					mockModuleHub.EXPECT().GetModules().Return(&types.Modules{PushKey: mockPushKeyModule})
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
					restApi.GetPushKeyOwnerNonce(w, req)
					_ = req.Body.Close()
					Expect(w.Code).To(Equal(tp.statusCode))
					Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
				})
			})
		}
	})
})
