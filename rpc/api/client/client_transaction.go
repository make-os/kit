package client

import (
	"gitlab.com/makeos/mosdef/types/api"
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
func (c *RPCClient) SendTxPayload(data map[string]interface{}) (*api.SendTxPayloadResponse, *util.StatusError) {
	out, statusCode, err := c.call("tx_sendPayload", data)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var result api.SendTxPayloadResponse
	_ = util.DecodeMap(out, &result)

	return &result, nil
}
