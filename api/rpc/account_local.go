package rpc

import (
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"
)

// LocalAccountAPI provides RPC methods for
// various local key management functionality.
type LocalAccountAPI struct {
	mods *modules.Modules
}

// NewLocalAccountAPI creates an instance of LocalAccountAPI
func NewLocalAccountAPI(mods *modules.Modules) *LocalAccountAPI {
	return &LocalAccountAPI{mods: mods}
}

// getAccount returns the account corresponding to the given address
// Response <map>:
// - accounts <[]string>: list of addresses
func (l *LocalAccountAPI) listAccounts(interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"accounts": l.mods.Account.ListLocalAccounts(),
	})
}

// APIs returns all API handlers
func (l *LocalAccountAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"listAccounts": {
			Namespace:   types.NamespaceUser,
			Private:     true,
			Description: "List all accounts that exist on the node",
			Func:        l.listAccounts,
		},
	}
}
