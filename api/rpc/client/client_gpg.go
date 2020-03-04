package client

import (
	"fmt"

	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// GPGGetAccountOfOwner returns the account of the owner of a given gpg public key
//
// ARGS:
// - id: The GPG public key unique ID
// - [blockHeight]: The target block height to query (default: latest).
//
// RETURNS:
// - resp <map> - state.Account
func (c *RPCClient) GPGGetAccountOfOwner(id string, blockHeight ...uint64) (*state.Account, *util.StatusError) {
	out, statusCode, err := c.call("gpg_getAccountOfOwner", util.Map{
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

// GPGGetNextNonceOfOwnerUsingRPCClient gets the next account nonce
// of the owner of the gpg key by querying the given JSON-RPC 2.0 client.
//
// ARGS:
// gpgID: The GPG public key ID
// client: The RPCClient to use
//
// RETURNS
// nonce: The next nonce of the account
func GPGGetNextNonceOfOwnerUsingRPCClient(gpgID string, client Client) (string, *util.StatusError) {
	acct, err := client.GPGGetAccountOfOwner(gpgID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", uint64(acct.Nonce+1)), nil
}
