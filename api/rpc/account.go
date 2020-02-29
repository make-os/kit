package rpc

import (
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/modules"
)

// AccountAPI provides RPC methods for various account related functionalities.
type AccountAPI struct {
	mods *modules.Modules
}

// NewAccountAPI creates an instance of AccountAPI
func NewAccountAPI(mods *modules.Modules) *AccountAPI {
	return &AccountAPI{mods: mods}
}

// getNonce returns the nonce of an account
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <string> - The account nonce
func (a *AccountAPI) getNonce(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)

	address, errResp := rpc.GetStringFromObjxMap(o, "address", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	nonce := a.mods.Account.GetNonce(address, blockHeight)

	return rpc.Success(nonce)
}

// getAccount returns the account corresponding to the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (a *AccountAPI) getAccount(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)

	address, errResp := rpc.GetStringFromObjxMap(o, "address", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	account := a.mods.Account.GetAccount(address, blockHeight)

	return rpc.Success(account)
}

// APIs returns all API handlers
func (a *AccountAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"getNonce": {
			Namespace:   types.NamespaceUser,
			Description: "Get the nonce of an account",
			Func:        a.getNonce,
		},
		"get": {
			Namespace:   types.NamespaceUser,
			Description: "Get the account corresponding to an address",
			Func:        a.getAccount,
		},
	}
}
