package rest

import (
	"net/http"

	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/util"
)

// GPGFind gets the GPG key associated with the given ID
// QueryParams:
// - id: The gpg key bech32 unique ID
// - [blockHeight]: The height of the block to query (default: latest)
// Response <map> - state.GPGPubKey
func (r *RESTApi) GPGFind(w http.ResponseWriter, req *http.Request) {
	query := objx.MustFromURLQuery(req.URL.Query().Encode())
	id := query.Get("id").String()

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(query, "blockHeight", false)
	if errResp != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg(errResp.Err.Message, "blockHeight", errResp.Err.Code))
		return
	}

	gpgKey := r.Modules().GPG.Get(id, blockHeight)
	util.WriteJSON(w, 200, util.StructToMap(gpgKey))
}

// GPGGetNonceOfOwner gets the account nonce of the gpg key owner
// QueryParams:
// - id: The gpg key bech32 unique ID
// - [blockHeight]: The height of the block to query (default: latest)
// Response <map>
// - nonce <string> The key owner account nonce
func (r *RESTApi) GPGGetOwnerNonce(w http.ResponseWriter, req *http.Request) {
	query := objx.MustFromURLQuery(req.URL.Query().Encode())
	id := query.Get("id").String()

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(query, "blockHeight", false)
	if errResp != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg(errResp.Err.Message, "blockHeight", errResp.Err.Code))
		return
	}

	acct := r.Modules().GPG.GetAccountOfOwner(id, blockHeight)

	util.WriteJSON(w, 200, map[string]interface{}{
		"nonce": acct["nonce"].(string),
	})
}
