package client

import (
	"fmt"
	"strconv"

	"gitlab.com/makeos/mosdef/util"
)

// AccountGet gets an account corresponding to a given address
//
// ARGS:
// - address <string>: The address of an account
// - [blockHeight] <string>: The target query block height (default: latest).
//
// RETURNS:
// - resp <map> - state.Account
func (c *RPCClient) AccountGet(address string, blockHeight ...uint64) (util.Map, error) {
	acct, err := c.Call("user_get", util.Map{
		"address":     address,
		"blockHeight": util.AtUint64Slice(0, blockHeight...),
	})
	if err != nil {
		return nil, err
	}
	return acct.(map[string]interface{}), nil
}

// AccountGetNextNonceUsingRPCClient gets the next account nonce
// of the owner of the gpg key by querying the given JSON-RPC 2.0 client.
//
// ARGS:
// address: The address of the account
// client: The RPCClient to use
//
// RETURNS:
// nonce: The next nonce of the account
func AccountGetNextNonceUsingRPCClient(address string, client *RPCClient) (string, error) {
	acct, err := client.AccountGet(address)
	if err != nil {
		return "", err
	}
	nonceStr := acct["nonce"].(string)
	nonce, _ := strconv.ParseUint(nonceStr, 10, 64)
	return fmt.Sprintf("%d", uint64(nonce+1)), nil
}
