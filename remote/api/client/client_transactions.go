package client

import (
	"fmt"
	"net/http"

	"gitlab.com/makeos/mosdef/remote/api"
	apitypes "gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/constants"
)

// SendTxPayload sends a signed transaction to the mempool
func (c *ClientV1) SendTxPayload(data map[string]interface{}) (*apitypes.SendTxPayloadResponse, error) {
	resp, err := c.post(api.V1Path(constants.NamespaceTx, apitypes.MethodNameSendPayload), data)
	if err != nil {
		return nil, err
	}

	if resp.Response().StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(resp.String())
	}

	var result apitypes.SendTxPayloadResponse
	return &result, resp.ToJSON(&result)
}
