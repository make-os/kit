package api

import (
	"net/http"

	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/util"
)

// GetAccountNonce handles request for getting the nonce of an account
func (r *API) GetAccountNonce(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())

	address := body.Get("address").Str()
	blockHeight := cast.ToUint64(body.Get("height").Inter())

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.Modules().Account.GetNonce(address, blockHeight),
	})
}

// Get handles request for getting an account
func (r *API) GetAccount(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())

	address := body.Get("address").Str()
	blockHeight := cast.ToUint64(body.Get("height").Inter())
	acct := r.Modules().Account.GetAccount(address, blockHeight)

	util.WriteJSON(w, 200, acct)
}
