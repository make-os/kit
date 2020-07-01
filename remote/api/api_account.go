package api

import (
	"net/http"

	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/util"
)

// GetAccountNonce handles request for getting the nonce of an account
// QueryParams:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response <map>
// - nonce <string> - The account nonce
func (r *API) GetAccountNonce(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())

	address, errResp := rpc.GetStringFromObjxMap(body, "address", true)
	if errResp != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg(errResp.Err.Message, "address", errResp.Err.Code))
		return
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(body, "blockHeight", false)
	if errResp != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg(errResp.Err.Message, "blockHeight", errResp.Err.Code))
		return
	}

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.Modules().Account.GetNonce(address, blockHeight),
	})
}

// Get handles request for getting an account
// QueryParams:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (r *API) GetAccount(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())

	address, errResp := rpc.GetStringFromObjxMap(body, "address", true)
	if errResp != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg(
			errResp.Err.Message, "address", errResp.Err.Code))
		return
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(body, "blockHeight", false)
	if errResp != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg(
			errResp.Err.Message, "blockHeight", errResp.Err.Code))
		return
	}

	acct := r.Modules().Account.GetAccount(address, blockHeight)

	util.WriteJSON(w, 200, acct)
}
