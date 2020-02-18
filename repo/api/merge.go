package api

import (
	"encoding/json"
	"net/http"

	"github.com/makeos/mosdef/modules"

	"github.com/makeos/mosdef/util"
)

// CreateMergeRequest creates a merge request proposal
func (r *Rest) CreateMergeRequest(w http.ResponseWriter, req *http.Request) {

	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", 0))
		return
	}

	resp := r.mods.GetModules().(*modules.Modules).
		Repo.CreateMergeRequest(body)

	util.WriteJSON(w, 201, resp)
}
