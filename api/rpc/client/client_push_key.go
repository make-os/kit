package client

import (
	"fmt"

	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// PushKeyGetAccountOfOwner returns the account that owns a push key
//
// ARGS:
// - id: The push key unique ID
// - [blockHeight]: The target block height to query (default: latest).
//
// RETURNS:
// - resp <map> - state.Account
func (c *RPCClient) PushKeyGetAccountOfOwner(id string, blockHeight ...uint64) (*state.Account, *util.StatusError) {
	out, statusCode, err := c.call("key_getAccountOfOwner", util.Map{
		"id":          id,
		"blockHeight": util.GetIndexFromUInt64Slice(0, blockHeight...)})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	acct := state.BareAccount()
	if err = acct.FromMap(out); err != nil {
		return nil, makeClientStatusErr("failed to decode call response: %s", err)
	}

	return acct, nil
}

// GetNextNonceOfPushKeyOwnerUsingRPCClient gets the next account nonce
// of the owner of the push key by querying the given JSON-RPC 2.0 client.
//
// ARGS:
// pushKeyID: The push key ID
// client: The RPCClient to use
//
// RETURNS
// nonce: The next nonce of the account
func GetNextNonceOfPushKeyOwnerUsingRPCClient(pushKeyID string, client Client) (string, *util.StatusError) {
	acct, err := client.PushKeyGetAccountOfOwner(pushKeyID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", acct.Nonce+1), nil
}
