package client

import (
	"gitlab.com/makeos/mosdef/remote/api"
	apitypes "gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
)

// GetAccountNonce returns the nonce of the given address
// Body:
// - address <string>: The address of the account
// - [height] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *ClientV1) GetAccountNonce(address string, blockHeight ...uint64) (*apitypes.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := api.V1Path(constants.NamespaceUser, apitypes.MethodNameGetNonce)
	resp, err := c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var result apitypes.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetAccount returns the account corresponding to the given address
// Body:
// - address <string>: The address of the account
// - [height] <string>: The target query block height (default: latest).
// Response:
// - resp <state.Account -> map> - The account object
func (c *ClientV1) GetAccount(address string, blockHeight ...uint64) (*apitypes.GetAccountResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := api.V1Path(constants.NamespaceUser, apitypes.MethodNameGetAccount)
	resp, err := c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var acct = &apitypes.GetAccountResponse{Account: state.BareAccount()}
	if err = resp.ToJSON(acct.Account); err != nil {
		return nil, err
	}

	return acct, nil
}
