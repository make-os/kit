package client

import (
	"fmt"
	"net/http"

	"github.com/make-os/lobe/api/remote"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/types/constants"
)

// SendTxPayload sends a signed transaction to the mempool
func (c *ClientV1) SendTxPayload(data map[string]interface{}) (*types.HashResponse, error) {
	resp, err := c.post(remote.V1Path(constants.NamespaceTx, types.MethodNameSendPayload), data)
	if err != nil {
		return nil, err
	}

	if resp.Response().StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(resp.String())
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}

// GetTransaction gets a transaction by hash
func (c *ClientV1) GetTransaction(hash string) (*types.GetTxResponse, error) {

	path := remote.V1Path(constants.NamespaceTx, types.MethodNameGetTx)
	resp, err := c.get(path, M{"hash": hash})
	if err != nil {
		return nil, err
	}

	var res types.GetTxResponse
	if err = resp.ToJSON(&res); err != nil {
		return nil, err
	}

	return &res, nil
}
