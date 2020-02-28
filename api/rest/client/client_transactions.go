package client

import (
	"fmt"
	"net/http"

	"gitlab.com/makeos/mosdef/api/rest"
	apitypes "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types"
)

// TxSendPayload sends a signed transaction to the mempool
func (c *RESTClient) TxSendPayload(data map[string]interface{}) (*apitypes.TxSendPayloadResponse, error) {
	resp, err := c.PostCall(rest.V1Path(types.NamespaceTx, rest.MethodNameSendPayload), data)
	if err != nil {
		return nil, err
	}

	if resp.Response().StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(resp.String())
	}

	var result apitypes.TxSendPayloadResponse
	return &result, resp.ToJSON(&result)
}

// TxSendPayloadUsingClients sends a signed transaction to the mempool using
// one of several Remote API clients until one succeeds.
func TxSendPayloadUsingClients(clients []*RESTClient, data map[string]interface{}) (*apitypes.TxSendPayloadResponse, error) {
	var err error
	for _, cl := range clients {
		var resp *apitypes.TxSendPayloadResponse
		resp, err = cl.TxSendPayload(data)
		if err != nil {
			continue
		}
		return resp, nil
	}
	return nil, err
}
