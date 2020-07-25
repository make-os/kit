package remote

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/pkgs/logger"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("API", func() {
	var ctrl *gomock.Controller
	var log logger.Logger

	BeforeEach(func() {
		log = logger.NewLogrusNoOp()
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".V1Path", func() {
		It("should return /v1/my_namespace/my_method when namespace='my_namespace' and method='my_method", func() {
			path := V1Path("my_namespace", "my_method")
			Expect(path).To(Equal("/v1/my_namespace/my_method"))
		})
	})

	Describe(".RegisterEndpoints", func() {
		It("should register all handlers", func() {
			api := &API{}
			mockMux := mocks.NewMockServeMux(ctrl)
			mockMux.EXPECT().HandleFunc(gomock.Any(), gomock.Any()).Times(12)
			api.RegisterEndpoints(mockMux)
		})
	})

	Describe(".APIHandler", func() {
		It("should set status to 405 if method is unexpected", func() {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/path/", bytes.NewBuffer(nil))
			APIHandler("POST", log, func(w http.ResponseWriter, r *http.Request) {})(w, r)
			Expect(w.Code).To(Equal(405))
		})

		It("should call handler", func() {
			called := false
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/path/", bytes.NewBuffer(nil))
			APIHandler("GET", log, func(w http.ResponseWriter, r *http.Request) {
				called = true
			})(w, r)
			Expect(called).To(BeTrue())
		})

		It("should recover from handler panic", func() {
			called := false
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/path/", bytes.NewBuffer(nil))
			APIHandler("GET", log, func(w http.ResponseWriter, r *http.Request) {
				called = true
				panic("error")
			})(w, r)
			Expect(called).To(BeTrue())
			Expect(w.Code).To(Equal(500))
		})

		It("should recover from handler panic (where error is a ReqError)", func() {
			called := false
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/path/", bytes.NewBuffer(nil))
			APIHandler("GET", log, func(w http.ResponseWriter, r *http.Request) {
				called = true
				panic(util.ReqErr(400, "", "", ""))
			})(w, r)
			Expect(called).To(BeTrue())
			Expect(w.Code).To(Equal(400))
		})
	})
})
