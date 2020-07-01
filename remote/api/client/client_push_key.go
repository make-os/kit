package client

import (
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/remote/api"
	apitypes "gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// GetPushKeyOwnerNonce returns the nonce of the push key owner account
// Body:
// - pushKeyID <string>: The push key ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <GetAccountNonceResponse>
func (c *ClientV1) GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*apitypes.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.get(api.V1Path(constants.NamespacePushKey, apitypes.MethodNameOwnerNonce), M{
		"id":          pushKeyID,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}

	var result apitypes.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetPushKey finds a push key by its ID
// Body:
// - pushKeyID <string>: The push key ID
// - [blockHeight] <string>: The target query block height (default: latest).
// Response:
// - resp <state.PushKey>
func (c *ClientV1) GetPushKey(pushKeyID string, blockHeight ...uint64) (*state.PushKey, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, err := c.get(api.V1Path(constants.NamespacePushKey, apitypes.MethodNamePushKeyFind), M{
		"id":          pushKeyID,
		"blockHeight": height,
	})
	if err != nil {
		return nil, err
	}

	var body map[string]interface{}
	_ = resp.ToJSON(&body)

	var pushKey state.PushKey
	_ = util.DecodeMap(body, &pushKey)

	pk, _ := crypto.PubKeyFromBase58(body["pubKey"].(string))
	pushKey.PubKey = pk.ToPublicKey()

	return &pushKey, nil
}
