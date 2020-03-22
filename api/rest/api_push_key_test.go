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
	"gitlab.com/makeos/mosdef/types/state"
)

var _ = Describe("GPG", func() {
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

	// TODO: fix this test
	// Describe(".PushKeyFind", func() {
	// 	var w *httptest.ResponseRecorder
	// 	var req *http.Request
	// 	var testCases = map[string]TestCase{
	// 		"id and blockHeight should be passed to PushKeyModule#Get": {
	// 			params:     map[string]string{"id": "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd", "blockHeight": "1"},
	// 			body:       `{"address":"maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc","pubKey":"-----BEGIN PGP PUBLIC KEY BLOCK..."}`,
	// 			statusCode: 200,
	// 			mocker: func(tc *TestCase) {
	// 				mockGPGModule := mocks.NewMockPushKeyModule(ctrl)
	// 				mockGPGModule.EXPECT().
	// 					Get("gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd", uint64(1)).
	// 					Return(&state.PushKey{
	// 						PubKey:  "-----BEGIN PGP PUBLIC KEY BLOCK...",
	// 						Address: "maker1ztejwuradar2tkk3pdu79txnn7f8g3qf8q6dcc"})
	// 				mockModuleHub.EXPECT().GetModules().Return(&modules2.Modules{GPG: mockGPGModule})
	// 			},
	// 		},
	// 	}
	//
	// 	for _tc, _tp := range testCases {
	// 		tc, tp := _tc, _tp
	// 		When(tc, func() {
	// 			It(fmt.Sprintf("should return statusCode=%d, msg=%s", tp.statusCode, tp.body), func() {
	// 				w = httptest.NewRecorder()
	// 				req = httptest.NewRequest("GET", "http://", nil)
	// 				q := req.URL.Query()
	// 				for k, v := range tp.params {
	// 					q.Add(k, v)
	// 				}
	//
	// 				if tp.mocker != nil {
	// 					tp.mocker(&tp)
	// 				}
	//
	// 				req.URL.RawQuery = q.Encode()
	// 				restApi.PushKeyFind(w, req)
	// 				_ = req.Body.Close()
	// 				Expect(w.Code).To(Equal(tp.statusCode))
	// 				Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
	// 			})
	// 		})
	// 	}
	// })

	Describe(".GetNonceOfPushKeyOwner", func() {
		var w *httptest.ResponseRecorder
		var req *http.Request
		var testCases = map[string]TestCase{
			"id and blockHeight should be passed to PushKeyModule#GetAccountOfOwner": {
				params:     map[string]string{"id": "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd", "blockHeight": "1"},
				body:       `{"nonce":"1000"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockGPGModule := mocks.NewMockPushKeyModule(ctrl)
					mockGPGModule.EXPECT().
						GetAccountOfOwner("gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd", uint64(1)).
						Return(&state.Account{Nonce: 1000})
					mockModuleHub.EXPECT().GetModules().Return(&modules2.Modules{PushKey: mockGPGModule})
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
					restApi.GetNonceOfPushKeyOwner(w, req)
					_ = req.Body.Close()
					Expect(w.Code).To(Equal(tp.statusCode))
					Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.body))
				})
			})
		}
	})
})
