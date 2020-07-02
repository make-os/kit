package client

import (
	"gitlab.com/makeos/mosdef/remote/api"
	apitypes "gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
)

// GetPushKeyOwnerNonce returns the nonce of the push key owner account
// Body:
// - pushKeyID <string>: The push key ID
// - [height] <string>: The target query block height (default: latest).
// Response:
// - resp <GetAccountNonceResponse>
func (c *ClientV1) GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*apitypes.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := c.get(api.V1Path(constants.NamespacePushKey, apitypes.MethodNameOwnerNonce), params)
	if err != nil {
		return nil, err
	}

	var result apitypes.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetPushKey finds a push key by its ID.
// If blockHeight is specified, only the block at the given height is searched.
func (c *ClientV1) GetPushKey(pushKeyID string, blockHeight ...uint64) (*apitypes.GetPushKeyResponse, error) {

	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := c.get(api.V1Path(constants.NamespacePushKey, apitypes.MethodNamePushKeyFind), params)
	if err != nil {
		return nil, err
	}

	var pk = &apitypes.GetPushKeyResponse{PushKey: state.BarePushKey()}
	if err = resp.ToJSON(pk.PushKey); err != nil {
		return nil, err
	}

	return pk, nil
}
