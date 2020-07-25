package rpc

import (
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	modtypes "gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/rpc"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/util"
)

// UserAPI provides RPC methods for various user related functionalities.
type UserAPI struct {
	mods *modtypes.Modules
}

// NewAccountAPI creates an instance of UserAPI
func NewAccountAPI(mods *modtypes.Modules) *UserAPI {
	return &UserAPI{mods: mods}
}

// getNonce returns the nonce of an account
func (a *UserAPI) getNonce(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	nonce := a.mods.User.GetNonce(address, blockHeight)
	return rpc.Success(util.Map{
		"nonce": nonce,
	})
}

// getAccount returns the account corresponding to the given address
func (a *UserAPI) getAccount(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	account := a.mods.User.GetAccount(address, blockHeight)
	return rpc.Success(account)
}

// sendCoin creates a transaction to transfer coin from a user account to a user/repo account.
func (a *UserAPI) sendCoin(params interface{}) (resp *rpc.Response) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a map", "")
	}
	return rpc.Success(a.mods.User.SendCoin(p))
}

// APIs returns all API handlers
func (a *UserAPI) APIs() rpc.APISet {
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
		{
			Name:        "send",
			Namespace:   constants.NamespaceUser,
			Description: "Send coins to another user account or a repository",
			Func:        a.sendCoin,
		},
	}
}
