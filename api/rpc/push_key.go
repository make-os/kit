package rpc

import (
	modulestypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/constants"
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

// registerPushKey creates a transaction to registers a public key as a push key
func (a *PushKeyAPI) registerPushKey(params interface{}) (resp *rpc.Response) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a map", "")
	}
	return rpc.Success(a.mods.PushKey.Register(p))
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
		{
			Name:        "register",
			Namespace:   constants.NamespacePushKey,
			Description: "Register a public key on the network",
			Func:        a.registerPushKey,
		},
	}
}
