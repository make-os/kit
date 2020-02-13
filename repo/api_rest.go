package repo

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/makeos/mosdef/modules"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/util"
)

func restAPIHandler(handler func(w http.ResponseWriter,
	r *http.Request), log logger.Logger, method string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				cause := errors.Cause(r.(error))
				util.WriteJSON(w, 500, util.RESTApiErrorMsg(cause.Error(), "", 0))
				log.Error("api handler error", "Err", cause.Error())
			}
		}()
		if strings.ToLower(r.Method) != strings.ToLower(method) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

type getNonceBody struct {
	Address string `json:"address"`
}

// apiGetNonce handles request for getting the nonce of an account
func (m *Manager) apiGetNonce(w http.ResponseWriter, r *http.Request) {

	var body getNonceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", 0))
		return
	}

	nonce := m.modulesAgg.GetModules().(*modules.Modules).
		Account.GetNonce(body.Address)

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": nonce,
	})
}

// apiCreateMergeRequest creates a merge request proposal
func (m *Manager) apiCreateMergeRequest(w http.ResponseWriter, r *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", 0))
		return
	}

	resp := m.modulesAgg.GetModules().(*modules.Modules).
		Repo.CreateMergeRequest(body)

	util.WriteJSON(w, 201, resp)
}
