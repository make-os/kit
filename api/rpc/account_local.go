package rpc

import (
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
)

// LocalAccountAPI provides RPC methods for various local account management functionality.
type LocalAccountAPI struct {
	mods *modules.Modules
}

// NewLocalAccountAPI creates an instance of LocalAccountAPI
func NewLocalAccountAPI(mods *modules.Modules) *LocalAccountAPI {
	return &LocalAccountAPI{mods: mods}
}

// getAccount returns the account corresponding to the given address
// Response:
// - resp - (Array<string>): list of addresses
func (l *LocalAccountAPI) listAccounts(interface{}) (resp *jsonrpc.Response) {
	return jsonrpc.Success(l.mods.Account.ListLocalAccounts())
}

// APIs returns all API handlers
func (l *LocalAccountAPI) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"listAccounts": {
			Namespace:   types.NamespaceUser,
			Private:     true,
			Description: "List all accounts that exist on the node",
			Func:        l.listAccounts,
		},
	}
}
