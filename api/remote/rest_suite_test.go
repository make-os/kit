package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type TestCase struct {
	params     map[string]string
	paramsRaw  []byte
	resp       string
	statusCode int
	mocker     func(tc *TestCase)
}

func testPostRequestCases(testCases map[string]TestCase, handler func(w http.ResponseWriter, req *http.Request)) {
	var w *httptest.ResponseRecorder
	var req *http.Request
	for _tc, _tp := range testCases {
		tc, tp := _tc, _tp
		When(tc, func() {
			It(fmt.Sprintf("should return statusCode=%d, msg=%s", tp.statusCode, tp.resp), func() {
				w = httptest.NewRecorder()
				var body []byte
				if tp.paramsRaw != nil {
					body = tp.paramsRaw
				}
				if tp.params != nil {
					body, _ = json.Marshal(tp.params)
				}
				req = httptest.NewRequest("POST", "http://", bytes.NewReader(body))
				if tp.mocker != nil {
					tp.mocker(&tp)
				}

				handler(w, req)
				_ = req.Body.Close()
				Expect(w.Code).To(Equal(tp.statusCode))
				Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.resp))
			})
		})
	}
}

func testGetRequestCases(testCases map[string]TestCase, handler func(w http.ResponseWriter, req *http.Request)) {
	var w *httptest.ResponseRecorder
	var req *http.Request
	for _tc, _tp := range testCases {
		tc, tp := _tc, _tp
		When(tc, func() {
			It(fmt.Sprintf("should return statusCode=%d, msg=%s", tp.statusCode, tp.resp), func() {
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
				handler(w, req)
				_ = req.Body.Close()
				Expect(w.Code).To(Equal(tp.statusCode))
				Expect(strings.TrimSpace(w.Body.String())).To(Equal(tp.resp))
			})
		})
	}
}

func TestRest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rest Suite")
}
