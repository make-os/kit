package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJSONRPC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "JSONRPC Suite")
}

var _ = Describe("RPC", func() {

	var rpc *Handler
	var cfg *config.AppConfig
	var mux *http.ServeMux

	BeforeEach(func() {
		cfg = config.EmptyAppConfig()
		cfg.RPC.On = true
		cfg.G().Log = logger.NewLogrusNoOp()
		mux = http.NewServeMux()
		rpc = New(mux, cfg)
	})

	Describe(".registerHandler", func() {
		It("should call registerHandler multiple time without panic", func() {
			Expect(func() {
				rpc.registerHandler(mux, "/rpc")
				rpc.registerHandler(mux, "/rpc")
			}).ToNot(Panic())
			Expect(rpc.handlerSet).To(BeTrue())
		})

		It("should return not set handler if RPC.ON is false", func() {
			cfg.RPC.On = false
			rpc = &Handler{cfg: cfg}
			rpc.registerHandler(mux, "/rpc")
			Expect(rpc.handlerSet).To(BeFalse())
		})
	})

	Describe(".handle", func() {
		It("should return nil and set CORS headers if method is OPTION", func() {
			data := []byte("{}")
			req, _ := http.NewRequest("OPTIONS", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(rr.Code).To(Equal(200))
				Expect(resp).To(BeNil())
				header := rr.Header()
				Expect(header.Get("Access-Control-Allow-Origin")).To(Equal("*"))
				Expect(header.Get("Access-Control-Allow-Methods")).To(Equal("POST, GET, OPTIONS, PUT, DELETE"))
				Expect(header.Get("Access-Control-Allow-Headers")).To(Equal("*"))
			})
			handler.ServeHTTP(rr, req)
		})

		It("should return 'Parse error' when json data is invalid", func() {
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
				Expect(rr.Code).To(Equal(200))
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
				Expect(rr.Code).To(Equal(200))
			})
			handler.ServeHTTP(rr, req)
		})

		It("should return method not found' when json rpc method is unknown", func() {
			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: "unknown"})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32601"))
				Expect(resp.Err.Message).To(Equal("method not found"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should return 'method not found' when json rpc method is not provided", func() {
			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: ""})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Err.Code).To(Equal("-32601"))
				Expect(resp.Err.Message).To(Equal("method not found"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should return 'method not found' error", func() {
			rpc.apiSet.Add(MethodInfo{
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
				Expect(resp.Err.Message).To(Equal("method not found"))
				Expect(resp.Result).To(BeNil())
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})

		Context("Successfully call method", func() {
			When("ID is added to the request body", func() {
				It("should return result", func() {
					rpc.apiSet.Add(MethodInfo{
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
						Params:         map[string]interface{}{"x": 2, "y": 2},
						ID:             1,
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
					rpc.apiSet.Add(MethodInfo{
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

		When("target method function is not a valid function type", func() {
			It("should return error of method function type is a string", func() {
				rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
					Func: "not function",
				})

				data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: "math_add", Params: map[string]interface{}{}, ID: 1})
				req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				rr := httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Code).To(Equal("50000"))
					Expect(resp.Err.Message).To(Equal("invalid method function signature"))
					Expect(rr.Code).To(Equal(200))
				})
				handler.ServeHTTP(rr, req)
			})

			It("should return error if method function signature is not a valid function", func() {
				rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
					Func: func() {},
				})

				data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: "math_add", Params: map[string]interface{}{}, ID: 1})
				req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				rr := httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Code).To(Equal("50000"))
					Expect(resp.Err.Message).To(Equal("invalid method function signature"))
					Expect(rr.Code).To(Equal(200))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("target method panicked with a standard go error", func() {
			It("should return error string", func() {
				rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
					Func: func(params interface{}) *Response {
						panic(fmt.Errorf("method panicked"))
					},
				})

				data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: "math_add", Params: map[string]interface{}{}, ID: 1})

				req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				rr := httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Code).To(Equal("50000"))
					Expect(resp.Err.Message).To(Equal("method panicked"))
					Expect(rr.Code).To(Equal(200))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("target method panicked with a string", func() {
			It("should return error string", func() {
				rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
					Func: func(params interface{}) *Response {
						panic("method panicked")
					},
				})

				data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", Method: "math_add", Params: map[string]interface{}{}, ID: 1})

				req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				rr := httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Code).To(Equal("50000"))
					Expect(resp.Err.Message).To(Equal("method panicked"))
					Expect(rr.Code).To(Equal(200))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("target method panicked with a ReqError", func() {
			It("should return error message=ReqError.Msg, status=ReqError.HttpCode, code=ReqError.Code, data=ReqError.Field", func() {
				err := errors.ReqErr(200, "some_code", "some_field", "field is bad")
				rpc.apiSet.Add(MethodInfo{
					Name:      "add",
					Namespace: "math",
					Func: func(params interface{}) *Response {
						panic(err)
					},
				})

				data, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "math_add",
					Params:         map[string]interface{}{},
					ID:             1,
				})

				req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
				rr := httptest.NewRecorder()
				rr.Header().Set("Content-Type", "application/json")
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp := rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Code).To(Equal(err.Code))
					Expect(resp.Err.Message).To(Equal(err.Msg))
					Expect(resp.Err.Data).To(Equal(err.Field))
					Expect(rr.Code).To(Equal(err.HttpCode))
				})
				handler.ServeHTTP(rr, req)
			})
		})

		When("`Sec-Websocket-Version` header was set", func() {
			It("should return error if body is not a valid JSON data", func() {
				var resp *Response
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					resp = rpc.handle(w, r)
					Expect(resp.Err).ToNot(BeNil())
					Expect(resp.Err.Code).To(Equal("-32700"))
					Expect(resp.Err.Message).To(Equal("Parse error"))
					Expect(resp.Result).To(BeNil())
				})
				s := httptest.NewServer(handler)
				u := "ws" + strings.TrimPrefix(s.URL, "http")
				ws, _, err := websocket.DefaultDialer.Dial(u, nil)
				Expect(err).To(BeNil())
				defer ws.Close()
				ws.WriteMessage(websocket.BinaryMessage, []byte("abc"))
			})

			It("should return result successfully", func() {
				rpc.apiSet.Add(MethodInfo{
					Name:      "add",
					Namespace: "math",
					Func: func(params interface{}) *Response {
						m := params.(map[string]interface{})
						return Success(util.Map{"result": m["x"].(float64) + m["y"].(float64)})
					},
				})

				body, _ := json.Marshal(Request{
					JSONRPCVersion: "2.0",
					Method:         "math_add",
					Params:         map[string]interface{}{"x": 2, "y": 2},
					ID:             1,
				})

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rpc.handle(w, r)
				})

				server := httptest.NewServer(handler)
				url := "ws" + strings.TrimPrefix(server.URL, "http")
				ws, _, err := websocket.DefaultDialer.Dial(url, nil)
				Expect(err).To(BeNil())
				defer ws.Close()

				ws.WriteMessage(websocket.BinaryMessage, body)
				_, msg, err := ws.ReadMessage()
				Expect(err).To(BeNil())

				var resp Response
				err = json.Unmarshal(msg, &resp)
				Expect(err).To(BeNil())
				Expect(resp.Result["result"]).To(Equal(4.0))
			})
		})
	})

	When("target method accepts a second CallContext argument", func() {
		It("should pass context to method with IsLocal=false", func() {
			rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
				Func: func(params interface{}, ctx *CallContext) *Response {
					Expect(ctx.IsLocal).To(BeFalse())
					return nil
				},
			})

			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", ID: "123", Method: "math_add", Params: map[string]interface{}{}})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).To(BeNil())
				Expect(resp.Result).To(BeNil())
				Expect(resp.ID).To(Equal("123"))
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})

		It("should pass context to method with IsLocal=true if RemoteAddr=127.0.0.1", func() {
			rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
				Func: func(params interface{}, ctx *CallContext) *Response {
					Expect(ctx.IsLocal).To(BeTrue())
					return nil
				},
			})

			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", ID: "123", Method: "math_add", Params: map[string]interface{}{}})
			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			req.RemoteAddr = "127.0.0.1:5555"
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).To(BeNil())
				Expect(resp.Result).To(BeNil())
				Expect(resp.ID).To(Equal("123"))
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})
	})

	When("target method returns nil response", func() {
		It("should return nil result", func() {
			rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
				Func: func(params interface{}) *Response { return nil },
			})

			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", ID: "123", Method: "math_add", Params: map[string]interface{}{}})

			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).To(BeNil())
				Expect(resp.Result).To(BeNil())
				Expect(resp.ID).To(Equal("123"))
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})
	})

	When("target method returns Error response", func() {
		It("should return nil result", func() {
			rpc.apiSet.Add(MethodInfo{Name: "add", Namespace: "math",
				Func: func(params interface{}) *Response { return Error("100", "bad error", "some_data") },
			})

			data, _ := json.Marshal(Request{JSONRPCVersion: "2.0", ID: "123", Method: "math_add", Params: map[string]interface{}{}})

			req, _ := http.NewRequest("POST", "/rpc", bytes.NewReader(data))
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", "application/json")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := rpc.handle(w, r)
				Expect(resp.Err).ToNot(BeNil())
				Expect(resp.Result).To(BeNil())
				Expect(resp.Err.Message).To(Equal("bad error"))
				Expect(resp.Err.Data).To(Equal("some_data"))
				Expect(resp.Err.Code).To(Equal("100"))
				Expect(rr.Code).To(Equal(200))
			})

			handler.ServeHTTP(rr, req)
		})
	})

	Context("Call private method", func() {
		When("rpc.disableauth=false, method is private and authorization is not set", func() {
			var req *http.Request
			var rr *httptest.ResponseRecorder
			BeforeEach(func() {
				cfg.RPC.DisableAuth = false

				rpc.apiSet.Add(MethodInfo{
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
					Expect(rr.Code).To(Equal(200))
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

				rpc.apiSet.Add(MethodInfo{
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
					Expect(rr.Code).To(Equal(200))
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

				rpc.apiSet.Add(MethodInfo{
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

				rpc.apiSet.Add(MethodInfo{
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
					Expect(rr.Code).To(Equal(200))
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

				rpc.apiSet.Add(MethodInfo{
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

	Describe(".MergeAPISet", func() {
		It("should add API", func() {
			apiSet1 := APISet([]MethodInfo{
				{Name: "add", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			apiSet2 := APISet([]MethodInfo{
				{Name: "add", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
				{Name: "div", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			rpc.MergeAPISet(apiSet1, apiSet2)
			Expect(rpc.apiSet).To(HaveLen(3))
		})
	})

	Describe(".Methods", func() {
		It("should return all methods name", func() {
			apiSet1 := APISet([]MethodInfo{
				{Name: "add", Namespace: "math", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			apiSet2 := APISet([]MethodInfo{
				{Name: "add", Namespace: "math", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
				{Name: "div", Namespace: "math", Func: func(params interface{}) *Response { return Success(util.Map{}) }},
			})
			rpc.MergeAPISet(apiSet1, apiSet2)
			m := rpc.Methods()
			Expect(m).To(HaveLen(3))
		})
	})
})
