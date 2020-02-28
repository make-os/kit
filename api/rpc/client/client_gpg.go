package client

import (
	"fmt"

	"github.com/pkg/errors"
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
func (c *RPCClient) GPGGetAccountOfOwner(id string, blockHeight ...uint64) (util.Map, error) {
	out, err := c.Call("gpg_getAccountOfOwner", util.Map{"id": id,
		"blockHeight": util.AtUint64Slice(0, blockHeight...)})
	if err != nil {
		return nil, err
	}
	return out.(map[string]interface{}), nil
}

// GPGGetNonceOfOwnerUsingRPCClient gets the next account nonce
// of the owner of the gpg key by querying the given JSON-RPC 2.0 client.
//
// ARGS:
// gpgID: The GPG public key ID
// client: The RPCClient to use
//
// RETURNS
// nonce: The next nonce of the account
func GPGGetNonceOfOwnerUsingRPCClient(gpgID string, client *RPCClient) (string, error) {
	out, err := client.GPGGetAccountOfOwner(gpgID)
	if err != nil {
		return "", errors.Wrap(err, "unable to query gpg key")
	}
	nonce := out["nonce"]
	return fmt.Sprintf("%d", uint64(nonce.(float64)+1)), nil
}
