package client

import (
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
)

// TxAPI provides access to the transaction-related RPC methods
type TxAPI struct {
	c *RPCClient
}

// Send sends a signed transaction payload to the mempool
func (t *TxAPI) Send(data map[string]interface{}) (*api.ResultHash, error) {
	out, statusCode, err := t.c.call("tx_sendPayload", data)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var result api.ResultHash
	if err = util.DecodeMap(out, &result); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &result, nil
}

// Get gets a transaction by its hash
func (t *TxAPI) Get(hash string) (*api.ResultTx, error) {
	resp, statusCode, err := t.c.call("tx_get", hash)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r api.ResultTx
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
