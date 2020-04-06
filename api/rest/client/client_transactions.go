package client

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api/rest"
	apitypes "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
)

// TxSendPayload sends a signed transaction to the mempool
func (c *Client) TxSendPayload(data map[string]interface{}) (*apitypes.TxSendPayloadResponse, error) {
	resp, err := c.post(rest.V1Path(constants.NamespaceTx, rest.MethodNameSendPayload), data)
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
func TxSendPayloadUsingClients(clients []RestClient, data map[string]interface{}) (*apitypes.TxSendPayloadResponse, error) {
	var errs []string
	for i, cl := range clients {
		var resp *apitypes.TxSendPayloadResponse
		resp, err := cl.TxSendPayload(data)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "client[%d]", i).Error())
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, ", "))
}
