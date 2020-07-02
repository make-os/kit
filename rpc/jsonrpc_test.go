package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RPC", func() {

	var rpc *JSONRPC
	var log = logger.NewLogrusNoOp()
	var cfg *config.AppConfig

	BeforeEach(func() {
		cfg = &config.AppConfig{
			RPC: &config.RPCConfig{},
		}
		rpc = newRPCServer("", cfg, log)
	})

	Describe(".handle", func() {
		It("should return 'Parse error' when json is invalid", func() {

			data := []byte("{,}")
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))

			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32700"))
				Expect(resp.Err.Message).To(Equal("Parse error"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(400))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should return '`jsonrpc` value is required' when json rpc version is invalid", func() {
			data, _ := json.Marshal(Request{})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))

			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32600"))
				Expect(resp.Err.Message).To(Equal("`jsonrpc` value is required"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(400))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should return Method not found' when json rpc method is unknown", func() {
			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: "unknown"})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))

			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32601"))
				Expect(resp.Err.Message).To(Equal("Method not found"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(404))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should return 'Method not found' when json rpc method is not provided", func() {
			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: ""})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))

			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32601"))
				Expect(resp.Err.Message).To(Equal("Method not found"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(404))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should return 'Method not found' error", func() {
			rpc.apiSet.Add(APIInfo{
				Name: "add",
				Func: func(params interface{}) *Response {
					return Success(nil)
				},
			})

			data, _ := json.Marshal(Request{
				JSONRPCVersion: "2.0",
				Method:         "plus",
				Params: map[string]interface{}{
					"x": 2, "y": 2,
				},
				ID: 1,
			})

			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32601"))
				Expect(resp.Err.Message).To(Equal("Method not found"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(404))
			})

			handler.ServeHTTP(rr, req)
		})

		Context("Successfully call method", func() {
			When("ID is added to the request body", func() {
				It("should return result", func() {
					rpc.apiSet.Add(APIInfo{
						Name:      "add",
						Namespace: "math",
						Func: func(params interface{}) *Response {
							m := params.(map[string]interface{})
							return Success(util.Map{"result": m["x"].(float64) + m["y"].(float64)})
						},
					})

					data, _ := json.Marshal(Request{
						JSONRPCVersion: "2.0",
						Method:         "math_add",
						Params: map[string]interface{}{
							"x": 2, "y": 2,
						},
						ID: 1,
					})

					req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))

					rr := httptest.NewRecorder()
					rr.Header().Set("Content-Type", "application/json")

					handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						resp := rpc.handle(w, r)
						Expect(resp.Err).To(BeNil())
						Expect(resp.Result).To(Equal(util.Map{"result": float64(4)}))
						Expect(resp.ID).To(Equal(float64(1)))
						Expect(rr.Code).To(Equal(200))
					})

					handler.ServeHTTP(rr, req)
				})
			})

			When("ID is not added to the request body", func() {
				It("should not return result", func() {
					rpc.apiSet.Add(APIInfo{
						Name:      "add",
						Namespace: "math",
						Func: func(params interface{}) *Response {
							m := params.(map[string]interface{})
							return Success(util.Map{"result": m["x"].(float64) + m["y"].(float64)})
						},
					})

					data, _ := json.Marshal(Request{
						JSONRPCVersion: "2.0",
						Method:         "math_add",
						Params: map[string]interface{}{
							"x": 2, "y": 2,
						},
					})

					req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))

					rr := httptest.NewRecorder()
					rr.Header().Set("Content-Type", "application/json")

					handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						resp := rpc.handle(w, r)
						Expect(resp.Err).To(BeNil())
						Expect(resp.Result).To(BeNil())
						Expect(resp.ID).To(BeZero())
						Expect(rr.Code).To(Equal(200))
					})

					handler.ServeHTTP(rr, req)
				})
			})
		})
	})

	Context("Call private method", func() {
		When("rpc.disableauth=false, method is private and authorization is not set", func() {
			var req *http.Request
			var rr *httptest.ResponseRecorder
			BeforeEach(func() {
				cfg.RPC.DisableAuth = false

				rpc.apiSet.Add(APIInfo{
					Name:      "echo",
					Private:   true,
					Namespace: "test",
					Func: func(params interface{}) *Response {
						return Success(util.Map{"result": params})
					},
				})

				data, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "test_echo",
					Params:         map[string]interface{}{},
				})

				req, _ = http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				rr = httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
			})

			It("should return error response", func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Message).To(Equal("basic authentication header is invalid"))
					Expect(resp.Err.Code).To(Equal(fmt.Sprintf("%d", types.ErrCodeInvalidAuthHeader)))
					Expect(rr.Code).To(Equal(401))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("rpc.disableauth=false, method is private and credentials are not valid", func() {
			var req *http.Request
			var rr *httptest.ResponseRecorder
			BeforeEach(func() {
				cfg.RPC.DisableAuth = false
				cfg.RPC.User = "correct_user"
				cfg.RPC.Password = "correct_pass"

				rpc.apiSet.Add(APIInfo{
					Name:      "echo",
					Private:   true,
					Namespace: "test",
					Func: func(params interface{}) *Response {
						return Success(util.Map{"result": params})
					},
				})

				data, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "test_echo",
					Params:         map[string]interface{}{},
				})

				req, _ = http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				req.SetBasicAuth("invalid", "invalid")
				rr = httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
			})

			It("should return error response", func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Message).To(Equal("authentication has failed. Invalid credentials"))
					Expect(resp.Err.Code).To(Equal(fmt.Sprintf("%d", types.ErrCodeInvalidAuthCredentials)))
					Expect(rr.Code).To(Equal(401))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("rpc.disableauth=true, method is private and credentials are not valid", func() {
			var req *http.Request
			var rr *httptest.ResponseRecorder
			BeforeEach(func() {
				cfg.RPC.DisableAuth = true
				cfg.RPC.User = "correct_user"
				cfg.RPC.Password = "correct_pass"

				rpc.apiSet.Add(APIInfo{
					Name:      "echo",
					Private:   true,
					Namespace: "test",
					Func: func(params interface{}) *Response {
						return Success(util.Map{"result": params})
					},
				})

				data, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "test_echo",
					Params:         map[string]interface{}{},
				})

				req, _ = http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				req.SetBasicAuth("invalid", "invalid")
				rr = httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
			})

			It("should return no error", func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).To(BeNil())
					Expect(rr.Code).To(Equal(200))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("rpc.disableauth=false, rpc.authpubmethod=true, method is public and credentials are not valid", func() {
			var req *http.Request
			var rr *httptest.ResponseRecorder
			BeforeEach(func() {
				cfg.RPC.DisableAuth = false
				cfg.RPC.AuthPubMethod = true
				cfg.RPC.User = "correct_user"
				cfg.RPC.Password = "correct_pass"

				rpc.apiSet.Add(APIInfo{
					Name:      "echo",
					Private:   false,
					Namespace: "test",
					Func: func(params interface{}) *Response {
						return Success(util.Map{"result": params})
					},
				})

				data, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "test_echo",
					Params:         map[string]interface{}{},
				})

				req, _ = http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				req.SetBasicAuth("invalid", "invalid")
				rr = httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
			})

			It("should return error response", func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Message).To(Equal("authentication has failed. Invalid credentials"))
					Expect(resp.Err.Code).To(Equal(fmt.Sprintf("%d", types.ErrCodeInvalidAuthCredentials)))
					Expect(rr.Code).To(Equal(401))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("rpc.disableauth=false, rpc.authpubmethod=false, method is public and credentials are not valid", func() {
			var req *http.Request
			var rr *httptest.ResponseRecorder
			BeforeEach(func() {
				cfg.RPC.DisableAuth = false
				cfg.RPC.AuthPubMethod = false
				cfg.RPC.User = "correct_user"
				cfg.RPC.Password = "correct_pass"

				rpc.apiSet.Add(APIInfo{
					Name:      "echo",
					Private:   false,
					Namespace: "test",
					Func: func(params interface{}) *Response {
						return Success(util.Map{"result": params})
					},
				})

				data, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "test_echo",
					Params:         map[string]interface{}{},
				})

				req, _ = http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				req.SetBasicAuth("invalid", "invalid")
				rr = httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
			})

			It("should return no error", func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).To(BeNil())
					Expect(rr.Code).To(Equal(200))
				})
				handler.ServeHTTP(rr, req)
			})
		})
	})

	Describe(".AddAPI", func() {
		It("should add API", func() {
			rpc.AddAPI(APIInfo{Name: "add", Namespace: "ns", Func: func(params interface{}) *Response { return Success(util.Map{}) }})
			Expect(rpc.apiSet).To(HaveLen(2))
			Expect(rpc.apiSet.Get("ns_add")).ToNot(BeNil())
		})
	})

	Describe(".MergeAPISet", func() {
		It("should add API", func() {
			apiSet1 := APISet([]APIInfo{
				{Name: "add", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			apiSet2 := APISet([]APIInfo{
				{Name: "add", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
				{Name: "div", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			rpc.MergeAPISet(apiSet1, apiSet2)
			Expect(rpc.apiSet).To(HaveLen(3))
		})
	})

	Describe(".Methods", func() {
		It("should return all methods name", func() {
			apiSet1 := APISet([]APIInfo{
				{Name: "add", Namespace: "math", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			apiSet2 := APISet([]APIInfo{
				{Name: "add", Namespace: "math", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
				{Name: "div", Namespace: "math", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			rpc.MergeAPISet(apiSet1, apiSet2)
			m := rpc.Methods()
			Expect(m).To(HaveLen(3))
		})
	})
})
