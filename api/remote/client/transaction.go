package client

import (
	"fmt"
	"net/http"

	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/types/constants"
)

// TxAPI provides access to the transaction-related remote APIs.
type TxAPI struct {
	c *RemoteClient
}

// Send sends a signed transaction to the mempool
func (c *TxAPI) Send(data map[string]interface{}) (*types.ResultHash, error) {
	resp, err := c.c.post(V1Path(constants.NamespaceTx, types.MethodNameSendPayload), data)
	if err != nil {
		return nil, err
	}

	if resp.Response().StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(resp.String())
	}

	var result types.ResultHash
	return &result, resp.ToJSON(&result)
}

// Get gets a transaction by hash
func (c *TxAPI) Get(hash string) (*types.ResultTx, error) {

	path := V1Path(constants.NamespaceTx, types.MethodNameGetTx)
	resp, err := c.c.get(path, M{"hash": hash})
	if err != nil {
		return nil, err
	}

	var res types.ResultTx
	if err = resp.ToJSON(&res); err != nil {
		return nil, err
	}

	return &res, nil
}
