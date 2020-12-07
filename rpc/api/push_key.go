package api

import (
	modulestypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// PushKeyAPI provides RPC methods for various push key functionality.
type PushKeyAPI struct {
	mods *modulestypes.Modules
}

// NewPushKeyAPI creates an instance of PushKeyAPI
func NewPushKeyAPI(mods *modulestypes.Modules) *PushKeyAPI {
	return &PushKeyAPI{mods: mods}
}

// find finds a push key by its address
func (a *PushKeyAPI) find(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	keyID := o.Get("id").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	key := a.mods.PushKey.Find(keyID, blockHeight)
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

// register creates a transaction to registers a public key as a push key
func (a *PushKeyAPI) register(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.PushKey.Register(cast.ToStringMap(params)))
}

// unregister creates a transaction to registers a public key as a push key
func (a *PushKeyAPI) unregister(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.PushKey.Unregister(cast.ToStringMap(params)))
}

// getByAddress returns a list of push key addresses owned by the given user address
func (a *PushKeyAPI) getByAddress(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"addresses": a.mods.PushKey.GetByAddress(cast.ToString(params)),
	})
}

// update updates a push key
func (a *PushKeyAPI) update(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.PushKey.Update(cast.ToStringMap(params)))
}

// APIs returns all API handlers
func (a *PushKeyAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:        "find",
			Namespace:   constants.NamespacePushKey,
			Func:        a.find,
			Description: "Find a push key",
		},
		{
			Name:        "getOwner",
			Namespace:   constants.NamespacePushKey,
			Func:        a.getOwner,
			Description: "Get the account of a push key owner",
		},
		{
			Name:        "register",
			Namespace:   constants.NamespacePushKey,
			Func:        a.register,
			Description: "Register a public key on the network",
		},
		{
			Name:        "unregister",
			Namespace:   constants.NamespacePushKey,
			Func:        a.unregister,
			Description: "Remove a public key from the network",
		},
		{
			Name:        "getByAddress",
			Namespace:   constants.NamespacePushKey,
			Func:        a.getByAddress,
			Description: "Get push keys belonging to a user address",
		},
		{
			Name:        "update",
			Namespace:   constants.NamespacePushKey,
			Func:        a.update,
			Description: "Update a push key",
		},
	}
}
