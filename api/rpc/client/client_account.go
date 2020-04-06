package client

import (
	"fmt"

	"gitlab.com/makeos/mosdef/types/state"
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
func (c *RPCClient) AccountGet(address string, blockHeight ...uint64) (*state.Account, *util.StatusError) {
	resp, statusCode, err := c.call("user_get", util.Map{
		"address":     address,
		"blockHeight": util.GetIndexFromUInt64Slice(0, blockHeight...),
	})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	acct := state.BareAccount()
	if err = acct.FromMap(resp); err != nil {
		return nil, makeClientStatusErr("failed to decode call response: %s", err)
	}

	return acct, nil
}

// GetNextNonceOfAccountUsingRPCClient gets the next account nonce
// of the owner of the gpg key by querying the given JSON-RPC 2.0 client.
//
// ARGS:
// address: The address of the account
// client: The RPCClient to use
//
// RETURNS:
// nonce: The next nonce of the account
func GetNextNonceOfAccountUsingRPCClient(address string, client Client) (string, *util.StatusError) {
	acct, err := client.AccountGet(address)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", acct.Nonce+1), nil
}
