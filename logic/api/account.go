package api

import (
	"github.com/makeos/mosdef/rpc/jsonrpc"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// AccountAPI provides RPC methods for various account related functionalities.
type AccountAPI struct {
	logic types.Logic
}

// NewAccountAPI creates an instance of AccountAPI
func NewAccountAPI(logic types.Logic) *AccountAPI {
	return &AccountAPI{logic: logic}
}

// getNonce returns the nonce of an account
func (a *AccountAPI) getNonce(params interface{}) *jsonrpc.Response {

	address, ok := params.(string)
	if !ok {
		err := types.ErrParamDecode("string")
		return jsonrpc.Error(types.ErrCodeInvalidParamType, err.Error(), nil)
	}

	account := a.logic.AccountKeeper().GetAccount(util.String(address))
	if account.IsEmpty() {
		return jsonrpc.Error(types.ErrCodeAccountNotFound, "account not found", nil)
	}

	return jsonrpc.Success(account.Nonce)
}

// APIs returns all API handlers
func (a *AccountAPI) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"getNonce": {
			Namespace:   types.NamespaceAccount,
			Description: "Get the nonce of an account",
			Func:        a.getNonce,
		},
	}
}
