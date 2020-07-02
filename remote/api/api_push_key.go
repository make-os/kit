package api

import (
	"net/http"

	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/util"
)

// GetPushKey finds a push key by ID
func (r *API) GetPushKey(w http.ResponseWriter, req *http.Request) {
	query := objx.MustFromURLQuery(req.URL.Query().Encode())
	id := query.Get("id").String()

	blockHeight := cast.ToUint64(query.Get("height").Inter())
	pk := r.Modules().PushKey.Get(id, blockHeight)

	util.WriteJSON(w, 200, pk)
}

// GetPushKeyOwnerNonce gets the account nonce of the push key owner
func (r *API) GetPushKeyOwnerNonce(w http.ResponseWriter, req *http.Request) {
	query := objx.MustFromURLQuery(req.URL.Query().Encode())
	id := query.Get("id").String()

	blockHeight := cast.ToUint64(query.Get("height").Inter())
	acct := r.Modules().PushKey.GetAccountOfOwner(id, blockHeight)

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": acct["nonce"],
	})
}
