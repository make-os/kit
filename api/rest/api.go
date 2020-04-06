package rest

import (
	errors2 "errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"
)

const (
	MethodNameGetNonce    = "get-nonce"
	MethodNameGetAccount  = "get-account"
	MethodNameSendPayload = "send-payload"
	MethodNameOwnerNonce  = "owner-nonce"
	MethodNamePushKeyFind = "find"
)

// API provides a REST API handlers
type API struct {
	mods modules.ModuleHub
	log  logger.Logger
}

// NewAPI creates an instance of API
func NewAPI(mods modules.ModuleHub, log logger.Logger) *API {
	return &API{mods: mods, log: log.Module("rest-api")}
}

// Modules returns modules
func (r *API) Modules() *modules.Modules {
	return r.mods.GetModules()
}

func (r *API) get(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return APIHandler("GET", handler, r.log)
}

func (r *API) post(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return APIHandler("POST", handler, r.log)
}

// RegisterEndpoints registers handlers to endpoints
func (r *API) RegisterEndpoints(s *http.ServeMux) {
	s.HandleFunc(V1Path(constants.NamespaceUser, MethodNameGetNonce), r.get(r.GetAccountNonce))
	s.HandleFunc(V1Path(constants.NamespaceUser, MethodNameGetAccount), r.get(r.GetAccount))
	s.HandleFunc(V1Path(constants.NamespaceTx, MethodNameSendPayload), r.post(r.TxSendPayload))
	s.HandleFunc(V1Path(constants.NamespacePushKey, MethodNameOwnerNonce), r.get(r.GetNonceOfPushKeyOwner))
	s.HandleFunc(V1Path(constants.NamespacePushKey, MethodNamePushKeyFind), r.get(r.FindPushKey))
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
