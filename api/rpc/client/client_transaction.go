package client

import (
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/util"
)

// SendTxPayload sends a signed transaction payload to the mempool
//
// ARGS:
// - data <string>: The GPG key unique ID
// - [height] <string>: The target query block height (default: latest).
//
// RETURNS:
// - resp <map> - state.Account
func (c *RPCClient) SendTxPayload(data map[string]interface{}) (*types.SendTxPayloadResponse, *util.StatusError) {
	out, statusCode, err := c.call("tx_sendPayload", data)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var result types.SendTxPayloadResponse
	_ = util.DecodeMap(out, &result)

	return &result, nil
}
