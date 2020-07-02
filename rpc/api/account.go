package api

import (
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

// AccountAPI provides RPC methods for various account related functionalities.
type AccountAPI struct {
	mods *types.Modules
}

// NewAccountAPI creates an instance of AccountAPI
func NewAccountAPI(mods *types.Modules) *AccountAPI {
	return &AccountAPI{mods: mods}
}

// getNonce returns the nonce of an account
func (a *AccountAPI) getNonce(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	nonce := a.mods.Account.GetNonce(address, blockHeight)
	return rpc.Success(util.Map{
		"nonce": nonce,
	})
}

// getAccount returns the account corresponding to the given address
func (a *AccountAPI) getAccount(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	account := a.mods.Account.GetAccount(address, blockHeight)
	return rpc.Success(account)
}

// APIs returns all API handlers
func (a *AccountAPI) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{
			Name:        "getNonce",
			Namespace:   constants.NamespaceUser,
			Description: "Get the nonce of an account",
			Func:        a.getNonce,
		},
		{
			Name:        "get",
			Namespace:   constants.NamespaceUser,
			Description: "Get the account corresponding to an address",
			Func:        a.getAccount,
		},
	}
}
