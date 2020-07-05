package client

import (
	"gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// GetPushKeyOwner gets the account that owns a push key
func (c *RPCClient) GetPushKeyOwner(id string, blockHeight ...uint64) (*api.GetAccountResponse, *util.StatusError) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	out, statusCode, err := c.call("pk_getOwner", util.Map{"id": id, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	r := &api.GetAccountResponse{Account: state.BareAccount()}
	if err = r.Account.FromMap(out); err != nil {
		return nil, util.StatusErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}
