package client

import (
	"gitlab.com/makeos/mosdef/api/rest"
	"gitlab.com/makeos/mosdef/types"
)

// AccountGetNonceResponse is the response of AccountGetNonce endpoint
type AccountGetNonceResponse struct {
	Nonce string `json:"nonce"`
}

// AccountGetNonce returns the nonce of the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *RESTClient) AccountGetNonce(address string, blockHeight ...uint64) (*AccountGetNonceResponse, error) {

	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.GetCall(rest.V1Path(types.NamespaceUser, rest.MethodNameGetNonce), M{
		"address":     address,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}
	var result AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}

// AccountGet returns the account corresponding to the given address
// Body:
// - address <string>: The address of the account
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *RESTClient) AccountGet(address string) (*AccountGetNonceResponse, error) {
	resp, err := c.GetCall(rest.V1Path(types.NamespaceUser, rest.MethodNameGetAccount), M{
		"address": address,
	})
	if err != nil {
		return nil, err
	}
	var result AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}
