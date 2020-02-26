package rpc

import (
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/api"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
)

// GPGAPI provides RPC methods for various gpg key functionality.
type GPGAPI struct {
	mods *modules.Modules
}

// NewGPGAPI creates an instance of GPGAPI
func NewGPGAPI(mod *modules.Modules) *GPGAPI {
	return &GPGAPI{mods: mod}
}

// find finds and returns a GPG public key by its key ID
// Body:
// - id <string>: The GPG key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <string> - The account nonce
func (a *GPGAPI) find(params interface{}) (resp *jsonrpc.Response) {
	o := objx.New(params)

	keyId, errResp := api.GetStringFromObjxMap(o, "id", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := api.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	key := a.mods.GPG.Find(keyId, blockHeight)
	return jsonrpc.Success(key)
}

// find finds and returns a GPG public key by its key ID
// Body:
// - id <string>: The GPG key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <string> - The account nonce
func (a *GPGAPI) getAccountOfOwner(params interface{}) (resp *jsonrpc.Response) {
	o := objx.New(params)

	keyId, errResp := api.GetStringFromObjxMap(o, "id", true)
	if errResp != nil {
		return errResp
	}

	blockHeight, errResp := api.GetStringToUint64FromObjxMap(o, "blockHeight", false)
	if errResp != nil {
		return errResp
	}

	account := a.mods.GPG.GetAccountOfOwner(keyId, blockHeight)

	return jsonrpc.Success(account)
}

// APIs returns all API handlers
func (a *GPGAPI) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"find": {
			Namespace:   types.NamespaceGPG,
			Description: "Find a GPG key by its key ID",
			Func:        a.find,
		},
		"getAccountOfOwner": {
			Namespace:   types.NamespaceGPG,
			Description: "Get the account of the owner of a gpg public key",
			Func:        a.getAccountOfOwner,
		},
	}
}
