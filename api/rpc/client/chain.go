package client

import (
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/util"
)

// ChainAPI implements Chain to provide access to the chain-related RPC methods
type ChainAPI struct {
	client *RPCClient
}

// GetBlock gets a block by height
func (c *ChainAPI) GetBlock(height uint64) (*types.BlockResult, error) {

	resp, statusCode, err := c.client.call("chain_getBlock", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.BlockResult
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}
