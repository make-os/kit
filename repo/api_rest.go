package repo

import (
	"encoding/json"
	"net/http"

	"github.com/makeos/mosdef/modules"
	"github.com/makeos/mosdef/util/logger"

	"github.com/makeos/mosdef/util"
)

func recoverableHandler(handler func(w http.ResponseWriter,
	r *http.Request), log logger.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				util.WriteJSON(w, 500, util.RESTApiErrorMsg(r.(error).Error(), "", 0))
				log.Error("api handler error", "Err", r.(error).Error())
			}
		}()
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

	nonce := m.modulesAgg.
		GetModules().(*modules.Modules).
		Account.GetNonce(body.Address)

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": nonce,
	})
}

// apiCreateMergeRequest creates a merge request proposal
func (m *Manager) apiCreateMergeRequest(w http.ResponseWriter, r *http.Request) {

}
