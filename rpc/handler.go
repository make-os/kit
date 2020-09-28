package rpc

import (
	"encoding/json"
	goerrors "errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/util"
	"github.com/pkg/errors"
)

// Handlers is responsible for handling incoming RPC requests
// by routing to a method that can handle the request and
// return a response.
type Handler struct {
	log logger.Logger
	cfg *config.AppConfig

	// apiSet is a collection of all known API methods
	apiSet APISet

	// handlerSet lets us know when the request handler has been configured
	handlerSet bool
}

// New creates an instance of Handler
func New(mux *http.ServeMux, cfg *config.AppConfig) *Handler {
	jsonrpc := &Handler{cfg: cfg, apiSet: APISet{}, log: cfg.G().Log.Module("json-rpc")}
	jsonrpc.MergeAPISet(jsonrpc.APIs())
	jsonrpc.registerHandler(mux, "/rpc")
	return jsonrpc
}

// APIs returns APIs for the jsonrpc package
func (s *Handler) APIs() APISet {
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
func (s *Handler) Methods() (methodsInfo []MethodInfo) {
	for _, api := range s.apiSet {
		methodsInfo = append(methodsInfo, api)
	}
	return
}

// registerHandler registers the main handler
func (s *Handler) registerHandler(mux *http.ServeMux, path string) {

	// Do not register handler if RPC service is not turned on
	if !s.cfg.RPC.On {
		return
	}

	if !s.handlerSet {
		mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resp := s.handle(w, r); resp != nil {
				_ = json.NewEncoder(w).Encode(resp)
			}
		}))
	}
	s.handlerSet = true
}

// HasAPI checks whether an API with matching full name exist
func (s *Handler) HasAPI(api MethodInfo) bool {
	for _, a := range s.apiSet {
		if a.FullName() == api.FullName() {
			return true
		}
	}
	return false
}

// MergeAPISet merges an API set with s current api sets
func (s *Handler) MergeAPISet(apiSets ...APISet) {
	for _, set := range apiSets {
		for _, v := range set {
			if !s.HasAPI(v) {
				s.apiSet = append(s.apiSet, v)
			}
		}
	}
}

// handle processes incoming requests. It validates
// the request according to JSON RPC specification,
// find and execute the target rpc method
func (s *Handler) handle(w http.ResponseWriter, r *http.Request) (resp *Response) {

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
		return Error(-32601, "method not found", nil)
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

		var respCode int

		// Check if a ReqError is the cause, then, we use the information
		// in the ReqError to create a good error response, otherwise we return
		// a less useful 500 error
		se := &util.ReqError{}
		cause := errors.Cause(err)
		if goerrors.As(cause, &se) {
			respCode = se.HttpCode
			resp = Error(se.Code, se.Msg, se.Field)
		} else {
			respCode = http.StatusInternalServerError
			resp = Error("unexpected_error", cause.Error(), "")
		}

		w.WriteHeader(respCode)

		// In dev mode, print out the stack for easy debugging
		if s.cfg.IsDev() {
			fmt.Println(string(debug.Stack()))
		}
	}()

	// Run the method
	funcVal := reflect.ValueOf(method.Func)
	if funcVal.Kind() == reflect.Func {

		params := reflect.ValueOf(newReq.Params)
		if newReq.Params == nil {
			params = reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem())
		}

		if funcVal.Type().ConvertibleTo(reflect.TypeOf((Method)(nil))) {
			resp = funcVal.Call([]reflect.Value{params})[0].Interface().(*Response)

		} else if funcVal.Type().ConvertibleTo(reflect.TypeOf((MethodWithContext)(nil))) {
			apiCtx := &CallContext{IsLocal: strings.HasPrefix(r.RemoteAddr, "127.0.0.1")}
			in := []reflect.Value{params, reflect.ValueOf(apiCtx)}
			resp = funcVal.Call(in)[0].Interface().(*Response)

		} else {
			panic(fmt.Errorf("invalid method function signature"))
		}

	} else {
		panic(fmt.Errorf("invalid method function signature"))
	}

	if resp == nil {
		resp = Success(nil)
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
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	return
}
