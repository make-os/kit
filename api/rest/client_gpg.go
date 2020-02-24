package rest

import (
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// GPGGetOwnerNonce returns the nonce of the gpg owner account
func (c *Client) GPGGetOwnerNonce(gpgKeyID string) (*AccountGetNonceResponse, error) {
	resp, err := c.GetCall(v1Path(types.NamespaceGPG, ownerNonceMethodName), M{
		"id": gpgKeyID,
	})
	if err != nil {
		return nil, err
	}
	var result AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// GPGFind finds a GPG public key matching the given ID
func (c *Client) GPGFind(gpgKeyID string) (*state.GPGPubKey, error) {
	resp, err := c.GetCall(v1Path(types.NamespaceGPG, gpgFindMethodName), M{
		"id": gpgKeyID,
	})
	if err != nil {
		return nil, err
	}
	var result state.GPGPubKey
	return &result, resp.ToJSON(&result)
}
