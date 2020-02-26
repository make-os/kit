package client

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/util"
)

// GPGGetAccountOfOwner returns the account of the owner of a given gpg public key
// Body:
// - address <string>: The address of an account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
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
func AccountGetNextNonceUsingRPCClient(address string, client *RPCClient) (string, error) {
	acct, err := client.AccountGet(address)
	if err != nil {
		return "", errors.Wrap(err, "unable to query account")
	}
	nonceStr := acct["nonce"].(string)
	nonce, _ := strconv.ParseUint(nonceStr, 10, 64)
	return fmt.Sprintf("%d", uint64(nonce+1)), nil
}
