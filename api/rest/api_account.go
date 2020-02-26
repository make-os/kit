package rest

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
// Response:
// - resp <string> - The account nonce
func (r *RESTApi) GetAccountNonce(w http.ResponseWriter, req *http.Request) {
	var body = objx.New(map[string]interface{}{})
	body.Set("address", req.URL.Query().Get("address"))
	body.Set("blockHeight", req.URL.Query().Get("blockHeight"))

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

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": r.Modules().Account.GetNonce(address, blockHeight),
	})
}

// GetAccount handles request for getting an account
// QueryParams:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (r *RESTApi) GetAccount(w http.ResponseWriter, req *http.Request) {
	var body = objx.New(map[string]interface{}{})
	body.Set("address", req.URL.Query().Get("address"))
	body.Set("blockHeight", req.URL.Query().Get("blockHeight"))

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
