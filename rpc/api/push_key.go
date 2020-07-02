package api

import (
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/constants"
)

// PushKeyAPI provides RPC methods for various push key functionality.
type PushKeyAPI struct {
	mods *types.Modules
}

// NewPushKeyAPI creates an instance of PushKeyAPI
func NewPushKeyAPI(mods *types.Modules) *PushKeyAPI {
	return &PushKeyAPI{mods: mods}
}

// find finds a push key by its ID
func (a *PushKeyAPI) find(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	keyID := o.Get("id").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	key := a.mods.PushKey.Get(keyID, blockHeight)
	return rpc.Success(key)
}

// getOwner gets the account of a given push key owner
func (a *PushKeyAPI) getOwner(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	keyID := o.Get("id").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	account := a.mods.PushKey.GetAccountOfOwner(keyID, blockHeight)
	return rpc.Success(account)
}

// APIs returns all API handlers
func (a *PushKeyAPI) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{
			Name:        "find",
			Namespace:   constants.NamespacePushKey,
			Description: "Find a push key",
			Func:        a.find,
		},
		{
			Name:        "getOwner",
			Namespace:   constants.NamespacePushKey,
			Description: "Get the account of a push key owner",
			Func:        a.getOwner,
		},
	}
}
