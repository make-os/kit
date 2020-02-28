package client

import (
	"fmt"
	"strconv"

	"gitlab.com/makeos/mosdef/api/rest"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// GPGGetNonceOfOwner returns the nonce of the gpg owner account
func (c *RESTClient) GPGGetNonceOfOwner(gpgID string) (*AccountGetNonceResponse, error) {
	resp, err := c.GetCall(rest.V1Path(types.NamespaceGPG, rest.MethodNameOwnerNonce), M{
		"id": gpgID,
	})
	if err != nil {
		return nil, err
	}
	var result AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// GPGFind finds a GPG public key matching the given ID
func (c *RESTClient) GPGFind(gpgID string) (*state.GPGPubKey, error) {
	resp, err := c.GetCall(rest.V1Path(types.NamespaceGPG, rest.MethodNameGPGFind), M{
		"id": gpgID,
	})
	if err != nil {
		return nil, err
	}
	var result state.GPGPubKey
	return &result, resp.ToJSON(&result)
}

// GPGGetNextNonceOfOwnerUsingClients gets the next account nonce of the owner of the
// gpg key by querying the given Remote API clients until one succeeds.
func GPGGetNextNonceOfOwnerUsingClients(clients []*RESTClient, gpgID string) (string, error) {
	var err error
	for _, cl := range clients {
		var resp *AccountGetNonceResponse
		resp, err = cl.GPGGetNonceOfOwner(gpgID)
		if err != nil {
			continue
		}
		nonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
		return fmt.Sprintf("%d", nonce+1), nil
	}
	return "", err
}
