package client

import (
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// GetPushKeyOwnerAccount returns the account that owns a push key
//
// ARGS:
// - id: The push key unique ID
// - [blockHeight]: The target block height to query (default: latest).
//
// RETURNS:
// - resp <map> - state.Account
func (c *RPCClient) GetPushKeyOwnerAccount(id string, blockHeight ...uint64) (*state.Account, *util.StatusError) {
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
