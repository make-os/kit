package services

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/imroc/req"
)

// TMRPC provides convenient access to specific tendermint RPC endpoints.
type TMRPC struct {
	req     *req.Req
	address string
}

// New creates an instance of TMRPC
func newTMRPC(address string) *TMRPC {
	return &TMRPC{
		req:     req.New(),
		address: address,
	}
}

// GetBlock returns the block at the given height
func (tm *TMRPC) getBlock(height int64) (map[string]interface{}, error) {

	var endpoint = fmt.Sprintf(`http://%s/block`, tm.address)
	if height > 0 {
		endpoint = fmt.Sprintf(`http://%s/block?height="%d"`, tm.address, height)
	}

	resp, err := tm.req.Get(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block")
	}

	if resp.Response().StatusCode == 500 {
		return nil, fmt.Errorf("failed to get block: server error")
	}

	var resData map[string]interface{}
	_ = resp.ToJSON(&resData)

	// If error, decode and return a simple error
	if resData["error"] != nil {
		errMsg := resData["error"].(map[string]interface{})["message"]
		errData := resData["error"].(map[string]interface{})["data"]
		return nil, fmt.Errorf("failed to get block: %s - %s", errMsg, errData)
	}

	return resData, nil
}
