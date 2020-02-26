package client

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/util"
)

// GetNextNonceOfGPGKeyOwnerUsingRPCClient gets the next account nonce
// of the owner of the gpg key by querying the given JSON-RPC 2.0 client.
func GetNextNonceOfGPGKeyOwnerUsingRPCClient(gpgID string, client *RPC) (string, error) {
	out, err := client.Call("gpg_getAccountOfOwner", util.Map{"id": gpgID})
	if err != nil {
		return "", errors.Wrap(err, "unable to query gpg key")
	}
	nonce := out.(map[string]interface{})["nonce"]
	return fmt.Sprintf("%d", uint64(nonce.(float64)+1)), nil
}
