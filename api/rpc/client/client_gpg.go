package client

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/util"
)

// GPGGetAccountOfOwner returns the account of the owner of a given gpg public key
// Body:
// - id <string>: The GPG key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <string> - The account nonce
func (c *RPCClient) GPGGetAccountOfOwner(id string, blockHeight ...uint64) (util.Map, error) {
	out, err := c.Call("gpg_getAccountOfOwner", util.Map{"id": id,
		"blockHeight": util.OptUint64(0, blockHeight...)})
	if err != nil {
		return nil, err
	}
	return out.(map[string]interface{}), nil
}

// GetNextNonceOfGPGKeyOwnerUsingRPCClient gets the next account nonce
// of the owner of the gpg key by querying the given JSON-RPCClient 2.0 client.
func GetNextNonceOfGPGKeyOwnerUsingRPCClient(gpgID string, client *RPCClient) (string, error) {
	out, err := client.GPGGetAccountOfOwner(gpgID)
	if err != nil {
		return "", errors.Wrap(err, "unable to query gpg key")
	}
	nonce := out["nonce"]
	return fmt.Sprintf("%d", uint64(nonce.(float64)+1)), nil
}
