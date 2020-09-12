package remote

import (
	errors2 "errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/make-os/lobe/api/remote/client"
	apitypes "github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/util"
	"github.com/pkg/errors"
)

type ServeMux interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// API provides a REST API handlers
type API struct {
	modules *types.Modules
	log     logger.Logger
}

// NewAPI creates an instance of API
func NewAPI(mods types.ModulesHub, log logger.Logger) *API {
	return &API{
		log:     log.Module("rest-api"),
		modules: mods.GetModules(),
	}
}

// Modules returns modules
func (r *API) Modules() *types.Modules {
	return r.modules
}

// get returns a handler for GET operations
func (r *API) get(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return APIHandler("GET", r.log, handler)
}

// get returns a handler for POST operations
func (r *API) post(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return APIHandler("POST", r.log, handler)
}

// RegisterEndpoints registers handlers to endpoints
func (r *API) RegisterEndpoints(s ServeMux) {
	s.HandleFunc(client.V1Path(constants.NamespaceUser, apitypes.MethodNameNonce), r.get(r.GetAccountNonce))
	s.HandleFunc(client.V1Path(constants.NamespaceUser, apitypes.MethodNameAccount), r.get(r.GetAccount))
	s.HandleFunc(client.V1Path(constants.NamespaceUser, apitypes.MethodNameSendCoin), r.post(r.SendCoin))
	s.HandleFunc(client.V1Path(constants.NamespaceTx, apitypes.MethodNameSendPayload), r.post(r.SendTxPayload))
	s.HandleFunc(client.V1Path(constants.NamespaceTx, apitypes.MethodNameGetTx), r.get(r.GetTransaction))
	s.HandleFunc(client.V1Path(constants.NamespacePushKey, apitypes.MethodNameOwnerNonce), r.get(r.GetPushKeyOwnerNonce))
	s.HandleFunc(client.V1Path(constants.NamespacePushKey, apitypes.MethodNamePushKeyFind), r.get(r.GetPushKey))
	s.HandleFunc(client.V1Path(constants.NamespacePushKey, apitypes.MethodNamePushKeyRegister), r.post(r.RegisterPushKey))
	s.HandleFunc(client.V1Path(constants.NamespaceRepo, apitypes.MethodNameCreateRepo), r.post(r.CreateRepo))
	s.HandleFunc(client.V1Path(constants.NamespaceRepo, apitypes.MethodNameGetRepo), r.get(r.GetRepo))
	s.HandleFunc(client.V1Path(constants.NamespaceRepo, apitypes.MethodNameAddRepoContribs), r.post(r.AddRepoContributors))
	s.HandleFunc(client.V1Path(constants.NamespaceRepo, apitypes.MethodNameRepoPropVote), r.post(r.RepoVote))
}

// APIHandler wraps http handlers, providing panic recovery
func APIHandler(method string, log logger.Logger, handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			if errMsg, ok := r.(string); ok {
				r = fmt.Errorf(errMsg)
			}

			cause := errors.Cause(r.(error))
			log.Error("api handler error", "Err", cause.Error())

			se := &util.ReqError{}
			if errors2.As(cause, &se) {
				util.WriteJSON(w, se.HttpCode, util.RESTApiErrorMsg(se.Msg, se.Field, se.Code))
			} else {
				util.WriteJSON(w, 500, util.RESTApiErrorMsg(cause.Error(), "", "0"))
			}
		}()

		if strings.ToLower(r.Method) != strings.ToLower(method) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprint(w, http.StatusText(http.StatusMethodNotAllowed))
			return
		}

		handler(w, r)
	}
}
