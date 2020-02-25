package rest

import "gitlab.com/makeos/mosdef/types"

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
func (c *Client) AccountGetNonce(address string) (*AccountGetNonceResponse, error) {
	resp, err := c.GetCall(v1Path(types.NamespaceUser, getNonceMethodName), M{
		"address": address,
	})
	if err != nil {
		return nil, err
	}
	var result AccountGetNonceResponse
	return &result, resp.ToJSON(&result)
}
