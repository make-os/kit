package rest

import (
	"net/http"

	"gitlab.com/makeos/mosdef/util"
)

// GPGFind gets the GPG key associated with the given ID
// QueryParams:
// - id: The gpg key bech32 unique ID
// Response
// - state.GPGPubKey
func (r *Rest) GPGFind(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	gpgKey := r.Modules().GPG.Find(id)
	util.WriteJSON(w, 200, util.StructToMap(gpgKey))
}

// GPGGetOwnerNonce gets the account nonce of the gpg key owner
// QueryParams:
// - id: The gpg key bech32 unique ID
// Response
// - nonce <string> The key owner account nonce
func (r *Rest) GPGGetOwnerNonce(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	gpgKey := r.Modules().GPG.Find(id)
	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.Modules().Account.GetNonce(gpgKey.Address.String()),
	})
}
