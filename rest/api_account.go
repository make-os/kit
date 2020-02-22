package rest

import (
	"net/http"

	"gitlab.com/makeos/mosdef/util"
)

type getNonceBody struct {
	Address string `json:"address"`
}

// GetAccountNonce handles request for getting the nonce of an account
// QueryParams:
// - address: The address of the account
// Response
// - nonce <string> The current nonce of the account.
func (r *Rest) GetAccountNonce(w http.ResponseWriter, req *http.Request) {
	var body getNonceBody
	body.Address = req.URL.Query().Get("address")
	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.Modules().Account.GetNonce(body.Address),
	})
}
