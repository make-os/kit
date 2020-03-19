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
	MethodNameGPGFind     = "find"
)

// RESTApi provides a REST API handlers
type RESTApi struct {
	mods modules.ModuleHub
	log  logger.Logger
}

// NewAPI creates an instance of RESTApi
func NewAPI(mods modules.ModuleHub, log logger.Logger) *RESTApi {
	return &RESTApi{mods: mods, log: log.Module("rest-api")}
}

// Modules returns modules
func (r *RESTApi) Modules() *modules.Modules {
	return r.mods.GetModules()
}

func (r *RESTApi) get(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return RESTApiHandler("GET", handler, r.log)
}

func (r *RESTApi) post(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return RESTApiHandler("POST", handler, r.log)
}

// RegisterEndpoints registers handlers to endpoints
func (r *RESTApi) RegisterEndpoints(s *http.ServeMux) {
	s.HandleFunc(RestV1Path(constants.NamespaceUser, MethodNameGetNonce), r.get(r.GetAccountNonce))
	s.HandleFunc(RestV1Path(constants.NamespaceUser, MethodNameGetAccount), r.get(r.GetAccount))
	s.HandleFunc(RestV1Path(constants.NamespaceTx, MethodNameSendPayload), r.post(r.TxSendPayload))
	s.HandleFunc(RestV1Path(constants.NamespacePushKey, MethodNameOwnerNonce), r.get(r.GPGGetOwnerNonce))
	s.HandleFunc(RestV1Path(constants.NamespacePushKey, MethodNameGPGFind), r.get(r.GPGFind))
}

// RestV1Path creates a REST API v1 path
func RestV1Path(ns, method string) string {
	return fmt.Sprintf("/v1/%s/%s", ns, method)
}

// RESTApiHandler wraps http handlers, providing panic recoverability
func RESTApiHandler(method string, handler func(w http.ResponseWriter,
	r *http.Request), log logger.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
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
