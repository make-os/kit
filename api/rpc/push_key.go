package rpc

import (
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

// find find a push key by its key ID
// Body:
// - id <string>: The push key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response <state.PushKey -> map>
func (a *PushKeyAPI) find(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)

	keyId, errResp := rpc.GetStringFromObjxMap(o, "id", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	key := a.mods.PushKey.Get(keyId, blockHeight)
	return rpc.Success(key)
}

// find finds and returns a push public key by its key ID
// Body:
// - id <string>: The push key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response <state.Account -> map>
func (a *PushKeyAPI) getAccountOfOwner(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)

	keyId, errResp := rpc.GetStringFromObjxMap(o, "id", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	account := a.mods.PushKey.GetAccountOfOwner(keyId, blockHeight)
	return rpc.Success(account)
}

// APIs returns all API handlers
func (a *PushKeyAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"find": {
			Namespace:   constants.NamespacePushKey,
			Description: "Find a push key",
			Func:        a.find,
		},
		"getAccountOfOwner": {
			Namespace:   constants.NamespacePushKey,
			Description: "Get the account that owns a push key",
			Func:        a.getAccountOfOwner,
		},
	}
}
