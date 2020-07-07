package client

import (
	"gitlab.com/makeos/mosdef/api/remote"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
)

// GetPushKeyOwnerNonce returns the nonce of the push key owner account
func (c *ClientV1) GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := c.get(remote.V1Path(constants.NamespacePushKey, types.MethodNameOwnerNonce), params)
	if err != nil {
		return nil, err
	}

	var result types.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetPushKey finds a push key by its ID.
// If blockHeight is specified, only the block at the given height is searched.
func (c *ClientV1) GetPushKey(pushKeyID string, blockHeight ...uint64) (*types.GetPushKeyResponse, error) {

	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := c.get(remote.V1Path(constants.NamespacePushKey, types.MethodNamePushKeyFind), params)
	if err != nil {
		return nil, err
	}

	var pk = &types.GetPushKeyResponse{PushKey: state.BarePushKey()}
	if err = resp.ToJSON(pk.PushKey); err != nil {
		return nil, err
	}

	return pk, nil
}
