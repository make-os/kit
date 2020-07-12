package remote

import (
	"encoding/json"
	"net/http"

	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/util"
)

// SendTxPayload sends a signed transaction to the mempool
func (r *API) SendTxPayload(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}
	util.WriteJSON(w, 201, r.Modules().Tx.SendPayload(body))
}

// GetTransaction queries a transaction by its hash
func (r *API) GetTransaction(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())
	hash := body.Get("hash").Str()
	tx := r.Modules().Tx.Get(hash)
	util.WriteJSON(w, 200, tx)
}
