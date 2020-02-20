package accountmgr

import (
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
)

func (am *AccountManager) apiListAccounts(interface{}) *jsonrpc.Response {
	accounts, err := am.ListAccounts()
	if err != nil {
		return jsonrpc.Error(types.ErrCodeUnexpected, err.Error(), nil)
	}

	var addresses []string
	for _, acct := range accounts {
		addresses = append(addresses, acct.Address)
	}

	return jsonrpc.Success(addresses)
}

// APIs returns all API handlers
func (am *AccountManager) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"listAccounts": {
			Namespace:   types.NamespaceAccount,
			Private:     true,
			Description: "List all accounts that exist on the node",
			Func:        am.apiListAccounts,
		},
	}
}
