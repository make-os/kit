package client

import (
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// GetAccount gets an account corresponding to a given address
func (c *RPCClient) GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, statusCode, err := c.call("user_get", util.Map{"address": address, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	r := &types.GetAccountResponse{Account: state.BareAccount()}
	if err = r.Account.FromMap(resp); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}
