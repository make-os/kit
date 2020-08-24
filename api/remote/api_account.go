package remote

import (
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
