package remote

import (
	"encoding/json"
	"net/http"

	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// GetAccountNonce handles request for getting the nonce of an account
func (r *API) GetAccountNonce(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())
	address := body.Get("address").Str()
	blockHeight := cast.ToUint64(body.Get("height").Inter())
	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.Modules().User.GetNonce(address, blockHeight),
	})
}

// Get handles request for getting an account
func (r *API) GetAccount(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())
	address := body.Get("address").Str()
	blockHeight := cast.ToUint64(body.Get("height").Inter())
	acct := r.Modules().User.GetAccount(address, blockHeight)
	util.WriteJSON(w, 200, acct)
}

// SendCoin handles request to send coins from a user account to another user or a repository.
func (r *API) SendCoin(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}
	util.WriteJSON(w, 201, r.modules.User.SendCoin(body))
}
