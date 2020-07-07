package client

import (
	"fmt"
	"net/http"

	"gitlab.com/makeos/mosdef/api/remote"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
)

// SendTxPayload sends a signed transaction to the mempool
func (c *ClientV1) SendTxPayload(data map[string]interface{}) (*types.SendTxPayloadResponse, error) {
	resp, err := c.post(remote.V1Path(constants.NamespaceTx, types.MethodNameSendPayload), data)
	if err != nil {
		return nil, err
	}

	if resp.Response().StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(resp.String())
	}

	var result types.SendTxPayloadResponse
	return &result, resp.ToJSON(&result)
}
