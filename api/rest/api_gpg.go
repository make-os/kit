package rest

import (
	"fmt"
	"net/http"

	"gitlab.com/makeos/mosdef/util"
)

// GPGFind gets the GPG key associated with the given ID
// QueryParams:
// - id: The gpg key bech32 unique ID
// Response
// - state.GPGPubKey
func (r *RESTApi) GPGFind(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	gpgKey := r.Modules().GPG.Find(id)
	util.WriteJSON(w, 200, util.StructToMap(gpgKey))
}

// GPGGetNonceOfOwner gets the account nonce of the gpg key owner
// QueryParams:
// - id: The gpg key bech32 unique ID
// Response
// - nonce <string> The key owner account nonce
func (r *RESTApi) GPGGetOwnerNonce(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	acct := r.Modules().GPG.GetAccountOfOwner(id)
	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": fmt.Sprintf("%d", acct.Nonce),
	})
}
