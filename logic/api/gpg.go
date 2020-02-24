package api

import (
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
)

// GPGAPI provides RPC methods for various gpg key functionalities.
type GPGAPI struct {
	logic core.Logic
}

// NewGPGAPI creates an instance of GPGAPI
func NewGPGAPI(logic core.Logic) *GPGAPI {
	return &GPGAPI{logic: logic}
}

// find finds a GPG key by its key ID
func (a *GPGAPI) find(params interface{}) *jsonrpc.Response {

	pkID, ok := params.(string)
	if !ok {
		err := types.ErrParamDecode("string")
		return jsonrpc.Error(types.RPCErrCodeInvalidParamType, err.Error(), nil)
	}

	key := a.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if key.IsNil() {
		return jsonrpc.Error(types.RPCErrCodeGPGKeyNotFound, "gpg key not found", nil)
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
