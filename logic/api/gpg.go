package api

import (
	types2 "gitlab.com/makeos/mosdef/logic/types"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
)

// GPGAPI provides RPC methods for various gpg key functionalities.
type GPGAPI struct {
	logic types2.Logic
}

// NewGPGAPI creates an instance of GPGAPI
func NewGPGAPI(logic types2.Logic) *GPGAPI {
	return &GPGAPI{logic: logic}
}

// find finds a GPG key by its key ID
func (a *GPGAPI) find(params interface{}) *jsonrpc.Response {

	pkID, ok := params.(string)
	if !ok {
		err := types.ErrParamDecode("string")
		return jsonrpc.Error(types.ErrCodeInvalidParamType, err.Error(), nil)
	}

	key := a.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if key.IsNil() {
		return jsonrpc.Error(types.ErrCodeGPGKeyNotFound, "gpg key not found", nil)
	}

	return jsonrpc.Success(key)
}

// APIs returns all API handlers
func (a *GPGAPI) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"find": {
			Namespace:   types.NamespaceGPG,
			Description: "Find a GPG key by its key ID",
			Func:        a.find,
		},
	}
}
