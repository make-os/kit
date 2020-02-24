package api

import (
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// AccountAPI provides RPC methods for various account related functionalities.
type AccountAPI struct {
	logic core.Logic
}

// NewAccountAPI creates an instance of AccountAPI
func NewAccountAPI(logic core.Logic) *AccountAPI {
	return &AccountAPI{logic: logic}
}

// getNonce returns the nonce of an account
// Body:
// - address <string> - The address of the account
// - [blockHeight] <string> - The target query block height (default: latest).
// Response:
// - string
func (a *AccountAPI) getNonce(params interface{}) *jsonrpc.Response {
	o := objx.New(params)

	address, errResp := jsonrpc.GetStringFromObjxMap(o, "address", false)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := jsonrpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	account := a.logic.AccountKeeper().GetAccount(util.String(address), blockHeight)
	if account.IsNil() {
		return jsonrpc.Error(types.RPCErrCodeAccountNotFound, "account not found", nil)
	}

	return jsonrpc.Success(account.Nonce)
}

// getAccount returns the account corresponding to the given address
// Body:
// - address <string> - The address of the account
// - [blockHeight] <string> - The target query block height (default: latest).
// Response:
// - state.Account
func (a *AccountAPI) getAccount(params interface{}) *jsonrpc.Response {
	o := objx.New(params)

	address, errResp := jsonrpc.GetStringFromObjxMap(o, "address", false)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := jsonrpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	account := a.logic.AccountKeeper().GetAccount(util.String(address), blockHeight)
	if account.IsNil() {
		return jsonrpc.Error(types.RPCErrCodeAccountNotFound, "account not found", nil)
	}

	return jsonrpc.Success(modules.EncodeForJS(account))
}

// APIs returns all API handlers
func (a *AccountAPI) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"getNonce": {
			Namespace:   types.NamespaceAccount,
			Description: "Get the nonce of an account",
			Func:        a.getNonce,
		},
		"getAccount": {
			Namespace:   types.NamespaceAccount,
			Description: "Get the account corresponding to an address",
			Func:        a.getAccount,
		},
	}
}
