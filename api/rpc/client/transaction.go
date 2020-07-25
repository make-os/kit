package client

import (
	"github.com/themakeos/lobe/api/types"
	"github.com/themakeos/lobe/util"
)

// SendTxPayload sends a signed transaction payload to the mempool
func (c *RPCClient) SendTxPayload(data map[string]interface{}) (*types.HashResponse, error) {
	out, statusCode, err := c.call("tx_sendPayload", data)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var result types.HashResponse
	_ = util.DecodeMap(out, &result)

	return &result, nil
}

// GetTransaction gets a transaction by its hash
func (c *RPCClient) GetTransaction(hash string) (map[string]interface{}, error) {
	resp, statusCode, err := c.call("tx_get", hash)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}
	var r map[string]interface{}
	_ = util.DecodeMap(resp, &r)
	return r, nil
}
