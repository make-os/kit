package rpc

import (
	"context"
	"encoding/json"
	errors2 "errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

const (
	middlewareErrCode = -32000
)

// MethodInfo describe an RPC method info
type MethodInfo struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

// OnRequestFunc is the type of function to use
// as a callback when newRPCServer requests are received
type OnRequestFunc func(r *http.Request) error

// Request represent a JSON RPC request
type Request struct {
	JSONRPCVersion string      `json:"jsonrpc"`
	Method         string      `json:"method"`
	Params         interface{} `json:"params"`
	ID             interface{} `json:"id,omitempty"`
}

// IsNotification checks whether the request is a notification
// according to JSON RPC specification.
// When ID is nil, we assume it's a notification request.
func (r Request) IsNotification() bool {
	if r.ID == nil {
		return true
	}

	switch v := r.ID.(type) {
	case string:
		return v == "0"
	case float64:
		return v == 0
	default:
		panic(fmt.Errorf("id has unexpected type"))
	}
}

// Err represents JSON RPC error object
type Err struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Response represents a JSON RPC response
type Response struct {
	JSONRPCVersion string      `json:"jsonrpc"`
	Result         util.Map    `json:"result"`
	Err            *Err        `json:"error,omitempty"`
	ID             interface{} `json:"id,omitempty"` // string or float64
}

// IsError checks whether r is an error response
func (r Response) IsError() bool {
	return r.Err != nil
}

// ToJSON returns the JSON encoding of r
func (r Response) ToJSON() []byte {
	bz, _ := json.Marshal(r)
	return bz
}

// JSONRPC defines a wrapper over mux json rpc
// that works with RPC functions of type `types.API`
// defined in packages that offer RPC APIs.`
type JSONRPC struct {
	log logger.Logger

	cfg *config.AppConfig

	// addr is the listening address
	addr string

	// apiSet is a collection of all known API methods
	apiSet APISet

	// onRequest is called before the request handler is called. If it returns
	// an error, the request handle is never called and the error is returned as
	// the request response.
	onRequest OnRequestFunc

	// handlerSet lets us know when the request handler has been configured
	handlerSet bool

	lck *sync.Mutex

	// server is the rpc server
	server *http.Server
}

// Error creates an error response
func Error(code interface{}, message string, data interface{}) *Response {
	return &Response{
		JSONRPCVersion: "2.0",
		Err:            &Err{Code: fmt.Sprintf("%v", code), Message: message, Data: data},
	}
}

// Success creates a success response
func Success(result util.Map) *Response {
	return &Response{JSONRPCVersion: "2.0", Result: result}
}

// newRPCServer creates a JSON-RPC 2.0 server
func newRPCServer(addr string, cfg *config.AppConfig, log logger.Logger) *JSONRPC {
	jsonrpc := &JSONRPC{
		cfg:    cfg,
		addr:   addr,
		apiSet: APISet{},
		log:    log.Module("json-rpc"),
		lck:    &sync.Mutex{},
		server: &http.Server{Addr: addr},
	}
	jsonrpc.MergeAPISet(jsonrpc.APIs())
	return jsonrpc
}

// APIs returns APIs for the jsonrpc package
func (s *JSONRPC) APIs() APISet {
	return APISet{
		{
			Name:        "methods",
			Description: "List RPC methods",
			Namespace:   constants.NamespaceRPC,
			Func: func(interface{}) *Response {
				return Success(util.Map{"methods": s.Methods()})
			},
		},
	}
}

// Methods gets the names of all methods in the API set.
func (s *JSONRPC) Methods() (methodsInfo []MethodInfo) {
	for _, api := range s.apiSet {
		methodsInfo = append(methodsInfo, MethodInfo{
			Name:        api.Name,
			Description: api.Description,
			Namespace:   api.Namespace,
			Private:     api.Private,
		})
	}
	return
}

// Serve starts the server
func (s *JSONRPC) Serve() {
	mux := http.NewServeMux()
	s.registerHandler(mux, "/")
	s.server.Handler = mux
	if err := s.server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			s.log.Fatal("Failed to start rpc server", "Err", err)
		}
	}
}

// registerHandler registers the main handler
func (s *JSONRPC) registerHandler(mux *http.ServeMux, path string) {
	if s.handlerSet {
		return
	}
	s.handlerSet = true
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.onRequest != nil {
			if err := s.onRequest(r); err != nil {
				_ = json.NewEncoder(w).Encode(Error(middlewareErrCode, err.Error(), nil))
				return
			}
		}

		// Handle the request.
		// When the response is non-nil, write it to the http writer.
		// Otherwise, do nothing as this indicates a panic occurred and in
		// such a case, the panic recovery function will write an appropriate
		// error response.
		if resp := s.handle(w, r); resp != nil {
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
}

// stop stops the RPC server
func (s *JSONRPC) stop() {
	s.lck.Lock()
	defer s.lck.Unlock()

	if s.server == nil {
		return
	}

	s.log.Debug("RPCServer is shutting down...")
	_ = s.server.Shutdown(context.Background())
	s.log.Debug("RPCServer has shutdown")
}

// HasAPI checks whether an API with matching full name exist
func (s *JSONRPC) HasAPI(api APIInfo) bool {
	for _, a := range s.apiSet {
		if a.FullName() == api.FullName() {
			return true
		}
	}
	return false
}

// MergeAPISet merges an API set with s current api sets
func (s *JSONRPC) MergeAPISet(apiSets ...APISet) {
	for _, set := range apiSets {
		for _, v := range set {
			if !s.HasAPI(v) {
				s.apiSet = append(s.apiSet, v)
			}
		}
	}
}

// AddAPI adds an API to s api set
func (s *JSONRPC) AddAPI(api APIInfo) {
	s.apiSet = append(s.apiSet, api)
}

// handle processes incoming requests. It validates
// the request according to JSON RPC specification,
// find and execute the target rpc method
func (s *JSONRPC) handle(w http.ResponseWriter, r *http.Request) *Response {

	var newReq Request
	if err := json.NewDecoder(r.Body).Decode(&newReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return Error(-32700, "Parse error", nil)
	}

	if newReq.JSONRPCVersion != "2.0" {
		w.WriteHeader(http.StatusBadRequest)
		return Error(-32600, "`jsonrpc` value is required", nil)
	}

	// Target method must be known
	method := s.apiSet.Get(newReq.Method)
	if method == nil {
		w.WriteHeader(http.StatusNotFound)
		return Error(-32601, "Method not found", nil)
	}

	if !s.cfg.RPC.DisableAuth && (method.Private || s.cfg.RPC.AuthPubMethod) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return Error(types.ErrCodeInvalidAuthHeader, "basic authentication header is invalid", nil)
		}
		if username != s.cfg.RPC.User || password != s.cfg.RPC.Password {
			w.WriteHeader(http.StatusUnauthorized)
			return Error(types.ErrCodeInvalidAuthCredentials, "authentication has failed. Invalid credentials", nil)
		}
	}

	var resp *Response

	// Recover from panics
	defer func() {
		rcv := recover()
		if rcv == nil {
			return
		}

		// Get error or convert non-err to error
		var err error
		if e, ok := rcv.(error); ok {
			err = e
		} else {
			err = fmt.Errorf("%v", rcv)
		}

		var resp *Response
		var respCode int

		// Check if a ReqError is the cause, then, we use the information
		// in the ReqError to create a good error response, otherwise we return
		// a less useful 500 error
		se := &util.ReqError{}
		cause := errors.Cause(err)
		if errors2.As(cause, &se) {
			respCode = se.HttpCode
			resp = Error(se.Code, se.Msg, se.Field)
		} else {
			respCode = http.StatusInternalServerError
			resp = Error("unexpected_error", cause.Error(), "")
		}

		w.WriteHeader(respCode)
		_ = json.NewEncoder(w).Encode(resp)

		// In dev mode, print out the stack for easy debugging
		if s.cfg.IsDev() {
			fmt.Println(string(debug.Stack()))
		}
	}()

	// Run the method
	resp = method.Func(newReq.Params)

	// If function result is nil return nil http response
	if resp == nil {
		w.WriteHeader(http.StatusOK)
		return Success(nil)
	}

	// If function returned no error.
	if !resp.IsError() {

		// Set RPC request ID
		resp.ID = newReq.ID

		// If request is a notification, send no response.
		if newReq.IsNotification() {
			resp.Result = nil
		}

		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}

	return resp
}
