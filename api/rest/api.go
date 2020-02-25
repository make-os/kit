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

// Rest provides a REST API handlers
type Rest struct {
	mods modtypes.ModulesAggregator
	log  logger.Logger
}

// New creates an instance of Rest
func New(mods modtypes.ModulesAggregator, log logger.Logger) *Rest {
	return &Rest{mods: mods, log: log.Module("rest-api")}
}

// Modules returns modules
func (r *Rest) Modules() *modules.Modules {
	return r.mods.GetModules().(*modules.Modules)
}

func v1Path(ns, method string) string {
	return fmt.Sprintf("/v1/%s/%s", ns, method)
}

func (r *Rest) get(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return util.RESTApiHandler("GET", handler, r.log)
}

func (r *Rest) post(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return util.RESTApiHandler("POST", handler, r.log)
}

const (
	getNonceMethodName    = "get-nonce"
	getAccountMethodName  = "get-account"
	sendPayloadMethodName = "send-payload"
	ownerNonceMethodName  = "owner-nonce"
	gpgFindMethodName     = "find"
)

// RegisterEndpoints registers handlers to endpoints
func (r *Rest) RegisterEndpoints(s *http.ServeMux) {
	s.HandleFunc(v1Path(types.NamespaceUser, getNonceMethodName), r.get(r.GetAccountNonce))
	s.HandleFunc(v1Path(types.NamespaceUser, getAccountMethodName), r.get(r.GetAccount))
	s.HandleFunc(v1Path(types.NamespaceTx, sendPayloadMethodName), r.post(r.SendTx))
	s.HandleFunc(v1Path(types.NamespaceGPG, ownerNonceMethodName), r.get(r.GPGGetOwnerNonce))
	s.HandleFunc(v1Path(types.NamespaceGPG, gpgFindMethodName), r.get(r.GPGFind))
}
