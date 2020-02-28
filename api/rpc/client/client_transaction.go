package client

import (
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/util"
)

// TxSendPayload sends a signed transaction payload to the mempool
//
// ARGS:
// - data <string>: The GPG key unique ID
// - [blockHeight] <string>: The target query block height (default: latest).
//
// RETURNS:
// - resp <map> - state.Account
func (c *RPCClient) TxSendPayload(data map[string]interface{}) (*types.TxSendPayloadResponse, error) {
	out, err := c.Call("tx_sendPayload", data)
	if err != nil {
		return nil, err
	}

	var result types.TxSendPayloadResponse
	_ = util.MapDecode(out.(map[string]interface{}), &result)

	return &result, nil
}
