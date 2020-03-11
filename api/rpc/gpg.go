package rpc

import (
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/modules"
)

// GPGAPI provides RPC methods for various gpg key functionality.
type GPGAPI struct {
	mods *modules.Modules
}

// NewGPGAPI creates an instance of GPGAPI
func NewGPGAPI(mods *modules.Modules) *GPGAPI {
	return &GPGAPI{mods: mods}
}

// find finds and returns a GPG public key by its key ID
// Body:
// - id <string>: The GPG key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response <state.GPGPubKey -> map>
func (a *GPGAPI) find(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)

	keyId, errResp := rpc.GetStringFromObjxMap(o, "id", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	key := a.mods.GPG.Get(keyId, blockHeight)
	return rpc.Success(key)
}

// find finds and returns a GPG public key by its key ID
// Body:
// - id <string>: The GPG key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response <state.Account -> map>
func (a *GPGAPI) getAccountOfOwner(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)

	keyId, errResp := rpc.GetStringFromObjxMap(o, "id", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := rpc.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	account := a.mods.GPG.GetAccountOfOwner(keyId, blockHeight)
	return rpc.Success(account)
}

// APIs returns all API handlers
func (a *GPGAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"find": {
			Namespace:   types.NamespaceGPG,
			Description: "Get a GPG key by its key ID",
			Func:        a.find,
		},
		"getAccountOfOwner": {
			Namespace:   types.NamespaceGPG,
			Description: "Get the account of the owner of a gpg public key",
			Func:        a.getAccountOfOwner,
		},
	}
}
