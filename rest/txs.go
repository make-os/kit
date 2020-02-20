package rest

import (
	"encoding/json"
	"net/http"

	"github.com/makeos/mosdef/modules"
	"github.com/makeos/mosdef/util"
)

// SendTx sends a signed transaction
func (r *Rest) SendTx(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", 0))
		return
	}
	util.WriteJSON(w, 201, r.mods.GetModules().(*modules.Modules).Tx.SendPayload(body))
}
