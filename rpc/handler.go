package rpc

import (
	"encoding/json"
	goerrors "errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	utilerrors "github.com/make-os/kit/util/errors"
	"github.com/pkg/errors"
)

// Handler is responsible for handling incoming RPC requests
// by routing to a method that can handle the request and
// return a response.
type Handler struct {
	log logger.Logger
	cfg *config.AppConfig

	// apiSet is a collection of all known API methods
	apiSet APISet

	// handlerSet lets us know when the request handler has been configured
	handlerSet bool

	upgrader *websocket.Upgrader
}

// New creates an instance of Handler
func New(mux *http.ServeMux, cfg *config.AppConfig) *Handler {
	jsonrpc := &Handler{
		log:        cfg.G().Log.Module("json-rpc"),
		cfg:        cfg,
		apiSet:     APISet{},
		handlerSet: false,
		upgrader:   &websocket.Upgrader{},
	}
	jsonrpc.MergeAPISet(jsonrpc.APIs())
	jsonrpc.registerHandler(mux, "/rpc")
	return jsonrpc
}

// APIs returns APIs for the jsonrpc package
func (s *Handler) APIs() APISet {
	return APISet{
		{
			Name:      "methods",
			Desc:      "List RPC methods",
			Namespace: constants.NamespaceRPC,
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
	if !s.cfg.RPC.On || s.handlerSet {
		return
	}
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handle(w, r)
	}))
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

// handler handles incoming JSONRPC 2.0 request over HTTP and Websocket.
func (s *Handler) handle(w http.ResponseWriter, r *http.Request) (resp *Response) {

	// Handle cors
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	var err error
	var c *websocket.Conn
	isWebSocket := r.Header.Get("Sec-Websocket-Version") != ""
	if isWebSocket {
		c, err = s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp = Error(-32603, "websocket upgrade failed", nil)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}

	writeResp := func() {
		if c != nil {
			c.WriteMessage(websocket.BinaryMessage, resp.ToJSON())
			return
		}
		json.NewEncoder(w).Encode(resp)
	}

	// Handle panics gracefully
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

		// Check if a ReqError is the cause, then, we use the information
		// in the ReqError to create a good error response, otherwise we return
		// a less useful 500 error
		se := &utilerrors.ReqError{}
		cause := errors.Cause(err)
		if goerrors.As(cause, &se) {
			resp = Error(se.Code, se.Msg, se.Field)
		} else {
			resp = Error(types.ErrRPCServerError, cause.Error(), "")
		}
		writeResp()
	}()

	useLoop := true
	for useLoop {
		// if not a websocket connection, cancel next loop.
		if c == nil {
			useLoop = false
		}

		var newReq Request
		if c != nil {
			_, message, err := c.ReadMessage()
			if err != nil {
				resp = Error(-32603, "failed to read message", nil)
				writeResp()
				break
			}
			if err := json.Unmarshal(message, &newReq); err != nil {
				resp = Error(-32700, "Parse error", nil)
				writeResp()
				break
			}
		} else {
			if err := json.NewDecoder(r.Body).Decode(&newReq); err != nil {
				return Error(-32700, "Parse error", nil)
			}
		}

		if newReq.JSONRPCVersion != "2.0" {
			resp = Error(-32600, "`jsonrpc` value is required", nil)
			writeResp()
			break
		}

		method := s.apiSet.Get(newReq.Method)
		if method == nil {
			resp = Error(-32601, "method not found", nil)
			writeResp()
			break
		}

		if !s.cfg.RPC.DisableAuth && (method.Private || s.cfg.RPC.AuthPubMethod) {
			username, password, ok := r.BasicAuth()
			if !ok {
				resp = Error(types.ErrCodeInvalidAuthHeader, "basic authentication header is invalid", nil)
				writeResp()
				break
			}
			if username != s.cfg.RPC.User || password != s.cfg.RPC.Password {
				resp = Error(types.ErrCodeInvalidAuthCredentials, "authentication has failed. Invalid credentials", nil)
				writeResp()
				break
			}
		}

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
				resp = Error(types.ErrRPCServerError, "invalid method function signature", nil)
				writeResp()
				break
			}
		} else {
			resp = Error(types.ErrRPCServerError, "invalid method function signature", nil)
			writeResp()
			break
		}

		if resp == nil {
			resp = Success(nil)
		}

		// If response from method is not an error, set the response ID or
		// remove the result if the request is a JSON-RPC 2.0 notification.
		if !resp.IsError() {
			resp.ID = newReq.ID
			if newReq.IsNotification() {
				resp.Result = nil
			}
		}
		writeResp()
	}

	return resp
}
