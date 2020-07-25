package rpc

import (
	"gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/rpc"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/util"
)

// LocalAccountAPI provides RPC methods for
// various local key management functionality.
type LocalAccountAPI struct {
	mods *types.Modules
}

// NewLocalAccountAPI creates an instance of LocalAccountAPI
func NewLocalAccountAPI(mods *types.Modules) *LocalAccountAPI {
	return &LocalAccountAPI{mods: mods}
}

// listAccount list all wallet accounts on the node
func (l *LocalAccountAPI) listAccounts(interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"accounts": l.mods.User.ListLocalAccounts(),
	})
}

// APIs returns all API handlers
func (l *LocalAccountAPI) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{
			Name:        "listAccounts",
			Namespace:   constants.NamespaceUser,
			Private:     true,
			Description: "List all accounts that exist on the node",
			Func:        l.listAccounts,
		},
	}
}
