package remote

import (
	"encoding/json"
	"net/http"

	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
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

// Register creates a transaction to register a public key
func (r *API) RegisterPushKey(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}

	util.WriteJSON(w, 201, r.Modules().PushKey.Register(body))
}
