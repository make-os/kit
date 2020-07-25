package remote

import (
	"encoding/json"
	"net/http"

	"gitlab.com/makeos/lobe/util"
)

// SendCoin handles request to send coins from a user account to another user or a repository.
func (r *API) SendCoin(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}
	util.WriteJSON(w, 201, r.modules.User.SendCoin(body))
}
