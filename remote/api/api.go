package api

import (
	errors2 "errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

// API provides a REST API handlers
type API struct {
	mods types.ModulesHub
	log  logger.Logger
}

// NewAPI creates an instance of API
func NewAPI(mods types.ModulesHub, log logger.Logger) *API {
	return &API{mods: mods, log: log.Module("rest-api")}
}

// Modules returns modules
func (r *API) Modules() *types.Modules {
	return r.mods.GetModules()
}

// get returns a handler for GET operations
func (r *API) get(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return APIHandler("GET", handler, r.log)
}

// get returns a handler for POST operations
func (r *API) post(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return APIHandler("POST", handler, r.log)
}

// RegisterEndpoints registers handlers to endpoints
func (r *API) RegisterEndpoints(s *http.ServeMux) {
	s.HandleFunc(V1Path(constants.NamespaceUser, api.MethodNameGetNonce), r.get(r.GetAccountNonce))
	s.HandleFunc(V1Path(constants.NamespaceUser, api.MethodNameGetAccount), r.get(r.GetAccount))
	s.HandleFunc(V1Path(constants.NamespaceTx, api.MethodNameSendPayload), r.post(r.SendTxPayload))
	s.HandleFunc(V1Path(constants.NamespacePushKey, api.MethodNameOwnerNonce), r.get(r.GetPushKeyOwnerNonce))
	s.HandleFunc(V1Path(constants.NamespacePushKey, api.MethodNamePushKeyFind), r.get(r.GetPushKey))
}

// V1Path creates a REST API v1 path
func V1Path(ns, method string) string {
	return fmt.Sprintf("/v1/%s/%s", ns, method)
}

// APIHandler wraps http handlers, providing panic recoverability
func APIHandler(method string, handler func(w http.ResponseWriter,
	r *http.Request), log logger.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {

				if errMsg, ok := r.(string); ok {
					r = fmt.Errorf(errMsg)
				}

				cause := errors.Cause(r.(error))
				log.Error("api handler error", "Err", cause.Error())

				se := &util.StatusError{}
				if errors2.As(cause, &se) {
					util.WriteJSON(w, se.HttpCode, util.RESTApiErrorMsg(se.Msg, se.Field, se.Code))
				} else {
					util.WriteJSON(w, 500, util.RESTApiErrorMsg(cause.Error(), "", "0"))
				}
			}
		}()
		if strings.ToLower(r.Method) != strings.ToLower(method) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}
