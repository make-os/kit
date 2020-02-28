package rest

import (
	"fmt"
	"net/http"

	"gitlab.com/makeos/mosdef/modules"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
)

// RESTApi provides a REST API handlers
type RESTApi struct {
	mods modtypes.ModulesAggregator
	log  logger.Logger
}

// NewAPI creates an instance of RESTApi
func NewAPI(mods modtypes.ModulesAggregator, log logger.Logger) *RESTApi {
	return &RESTApi{mods: mods, log: log.Module("rest-api")}
}

// Modules returns modules
func (r *RESTApi) Modules() *modules.Modules {
	return r.mods.GetModules().(*modules.Modules)
}

func V1Path(ns, method string) string {
	return fmt.Sprintf("/v1/%s/%s", ns, method)
}

func (r *RESTApi) get(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return util.RESTApiHandler("GET", handler, r.log)
}

func (r *RESTApi) post(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return util.RESTApiHandler("POST", handler, r.log)
}

const (
	MethodNameGetNonce    = "get-nonce"
	MethodNameGetAccount  = "get-account"
	MethodNameSendPayload = "send-payload"
	MethodNameOwnerNonce  = "owner-nonce"
	MethodNameGPGFind     = "find"
)

// RegisterEndpoints registers handlers to endpoints
func (r *RESTApi) RegisterEndpoints(s *http.ServeMux) {
	s.HandleFunc(V1Path(types.NamespaceUser, MethodNameGetNonce), r.get(r.GetAccountNonce))
	s.HandleFunc(V1Path(types.NamespaceUser, MethodNameGetAccount), r.get(r.GetAccount))
	s.HandleFunc(V1Path(types.NamespaceTx, MethodNameSendPayload), r.post(r.TxSendPayload))
	s.HandleFunc(V1Path(types.NamespaceGPG, MethodNameOwnerNonce), r.get(r.GPGGetOwnerNonce))
	s.HandleFunc(V1Path(types.NamespaceGPG, MethodNameGPGFind), r.get(r.GPGFind))
}
