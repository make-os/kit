package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
)

const (
	middlewareErrCode = -32000
	serverErrCode     = -32001
)

// MethodInfo describe an RPC method info
type MethodInfo struct {
	Name        string `json:"name"`
	Namespace   string `json:"-"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

// OnRequestFunc is the type of function to use
// as a callback when new requests are received
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
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Response represents a JSON RPC response
type Response struct {
	JSONRPCVersion string      `json:"jsonrpc"`
	Result         interface{} `json:"result"`
	Err            *Err        `json:"error,omitempty"`
	ID             interface{} `json:"id,omitempty"` // string or float64
}

// IsError checks whether r is an error response
func (r Response) IsError() bool {
	return r.Err != nil
}

// JSONRPC defines a wrapper over mux json rpc
// that works with RPC functions of type `types.API`
// defined in packages that offer RPC APIs.`
type JSONRPC struct {
	log logger.Logger

	cfg *config.RPCConfig

	// addr is the listening address
	addr string

	// apiSet is a collection of all known API methods
	apiSet APISet

	// onRequest is called before the request handler is called. If it returns
	// an error, the request handle is never called and the error is returned as
	// the request response.
	onRequest OnRequestFunc

	// handlerConfigured lets us know when the request handler has been configured
	handlerConfigured bool

	// server is the rpc server
	server *http.Server
}

// Error creates an error response
func Error(code int, message string, data interface{}) *Response {
	return &Response{
		JSONRPCVersion: "2.0",
		Err:            &Err{Code: code, Message: message, Data: data},
	}
}

// Success creates a success response
func Success(result interface{}) *Response {
	return &Response{JSONRPCVersion: "2.0", Result: result}
}

// New creates a JSONRPC server
func New(addr string, cfg *config.RPCConfig, log logger.Logger) *JSONRPC {
	rpc := &JSONRPC{
		cfg:    cfg,
		addr:   addr,
		apiSet: APISet{},
		log:    log.Module("jsonrpc"),
	}
	rpc.MergeAPISet(rpc.APIs())
	return rpc
}

// APIs returns APIs for the jsonrpc package
func (s *JSONRPC) APIs() APISet {
	return APISet{
		"methods": APIInfo{
			Description: "List RPC methods",
			Namespace:   "rpc",
			Func: func(interface{}) *Response {
				return Success(s.Methods())
			},
		},
	}
}

// Methods gets the names of all methods in the API set.
func (s *JSONRPC) Methods() (methodsInfo []MethodInfo) {
	for name, d := range s.apiSet {
		methodsInfo = append(methodsInfo, MethodInfo{
			Name:        name,
			Description: d.Description,
			Namespace:   d.Namespace,
			Private:     d.Private,
		})
	}
	return
}

// Serve starts the server
func (s *JSONRPC) Serve() {

	r := mux.NewRouter()
	server := rpc.NewServer()
	server.RegisterCodec(json2.NewCodec(), "application/json")
	server.RegisterCodec(json2.NewCodec(), "application/json;charset=UTF-8")
	r.Handle("/", server)

	s.server = &http.Server{Addr: s.addr}
	s.registerHandler()
	if err := s.server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			s.log.Fatal("Failed to start rpc server", "Err", err)
		}
	}
}

func (s *JSONRPC) registerHandler() {
	if s.handlerConfigured {
		return
	}
	http.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.onRequest != nil {
			if err := s.onRequest(r); err != nil {
				json.NewEncoder(w).Encode(Error(middlewareErrCode, err.Error(), nil))
				return
			}
		}
		json.NewEncoder(w).Encode(s.handle(w, r))
	}))
	s.handlerConfigured = true
}

// Stop stops the RPC server
func (s *JSONRPC) Stop() {
	if s.server == nil {
		return
	}
	s.log.Debug("Server is shutting down...")
	s.server.Shutdown(context.Background())
	s.log.Debug("Server has shutdown")
}

// MergeAPISet merges an API set with s current api sets
func (s *JSONRPC) MergeAPISet(apiSets ...APISet) {
	for _, set := range apiSets {
		for k, v := range set {
			s.apiSet[v.Namespace+"_"+k] = v
		}
	}
}

// makeFullAPIName returns the full API name used to map
// a RPC method to a server
func makeFullAPIName(namespace, apiName string) string {
	return fmt.Sprintf("%s_%s", namespace, apiName)
}

// AddAPI adds an API to s api set
func (s *JSONRPC) AddAPI(name string, api APIInfo) {
	s.apiSet[makeFullAPIName(api.Namespace, name)] = api
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
	f := s.apiSet.Get(newReq.Method)
	if f == nil {
		w.WriteHeader(http.StatusNotFound)
		return Error(-32601, "Method not found", nil)
	}

	if !s.cfg.DisableAuth && (f.Private || s.cfg.AuthPubMethod) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return Error(types.ErrCodeInvalidAuthHeader, "basic authentication header is invalid", nil)
		}
		if username != s.cfg.User || password != s.cfg.Password {
			w.WriteHeader(http.StatusUnauthorized)
			return Error(types.ErrCodeInvalidAuthCredentials, "authentication has failed. Invalid credentials", nil)
		}
	}

	var resp *Response

	defer func() {
		if rcv, ok := recover().(error); ok {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Error(serverErrCode, rcv.Error(), nil))
		}
	}()

	resp = f.Func(newReq.Params)
	if resp == nil {
		w.WriteHeader(http.StatusOK)
		return Success(nil)
	}

	if !resp.IsError() {
		resp.ID = newReq.ID

		// a notification. Send no response.
		if newReq.IsNotification() {
			resp.Result = nil
		}

		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}

	return resp
}
