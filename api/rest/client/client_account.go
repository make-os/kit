package client

import (
	"fmt"
	"strconv"

	"gitlab.com/makeos/mosdef/api/rest"
	apitypes "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types"
)

// AccountGetNonce returns the nonce of the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *RESTClient) AccountGetNonce(address string, blockHeight ...uint64) (*apitypes.AccountGetNonceResponse, error) {

	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.GetCall(rest.RestV1Path(types.NamespaceUser, rest.MethodNameGetNonce), M{
		"address":     address,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}
	var result apitypes.AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// AccountGet returns the account corresponding to the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *RESTClient) AccountGet(address string) (*apitypes.AccountGetNonceResponse, error) {
	resp, err := c.GetCall(rest.RestV1Path(types.NamespaceUser, rest.MethodNameGetAccount), M{
		"address": address,
	})
	if err != nil {
		return nil, err
	}
	var result apitypes.AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// AccountGetNextNonceUsingClients gets the next nonce of an account by
// querying the given Remote API clients until one succeeds.
func AccountGetNextNonceUsingClients(clients []*RESTClient, address string) (string, error) {
	var err error
	for _, cl := range clients {
		var resp *apitypes.AccountGetNonceResponse
		resp, err = cl.AccountGetNonce(address)
		if err != nil {
			continue
		}
		nonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
		return fmt.Sprintf("%d", nonce+1), nil
	}
	return "", err
}
