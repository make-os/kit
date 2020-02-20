package rest

import (
	"encoding/json"
	"net/http"

	"github.com/makeos/mosdef/modules"
	"github.com/makeos/mosdef/util"
)

type getNonceBody struct {
	Address string `json:"address"`
}

// GetAccountNonce handles request for getting the nonce of an account
func (r *Rest) GetAccountNonce(w http.ResponseWriter, req *http.Request) {
	var body getNonceBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", 0))
		return
	}
	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.mods.GetModules().(*modules.Modules).Account.GetNonce(body.Address),
	})
}
