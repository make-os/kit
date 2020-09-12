package client

import (
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/util"
)

// TxAPI provides access to the transaction-related RPC methods
type TxAPI struct {
	client *RPCClient
}

// Send sends a signed transaction payload to the mempool
func (t *TxAPI) Send(data map[string]interface{}) (*types.HashResponse, error) {
	out, statusCode, err := t.client.call("tx_sendPayload", data)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var result types.HashResponse
	_ = util.DecodeMap(out, &result)

	return &result, nil
}

// Get gets a transaction by its hash
func (t *TxAPI) Get(hash string) (*types.GetTxResponse, error) {
	resp, statusCode, err := t.client.call("tx_get", hash)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}
	var r types.GetTxResponse
	_ = util.DecodeMap(resp, &r)
	return &r, nil
}
