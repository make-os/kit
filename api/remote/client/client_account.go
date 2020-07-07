package client

import (
	"gitlab.com/makeos/mosdef/api/remote"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
)

// GetAccountNonce returns the nonce of the given address
func (c *ClientV1) GetAccountNonce(address string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := remote.V1Path(constants.NamespaceUser, types.MethodNameNonce)
	resp, err := c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var result types.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetAccount returns the account corresponding to the given address
func (c *ClientV1) GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := remote.V1Path(constants.NamespaceUser, types.MethodNameAccount)
	resp, err := c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var acct = &types.GetAccountResponse{Account: state.BareAccount()}
	if err = resp.ToJSON(acct.Account); err != nil {
		return nil, err
	}

	return acct, nil
}
