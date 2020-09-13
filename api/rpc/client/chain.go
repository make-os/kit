package client

import (
	"github.com/k0kubun/pp"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/util"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
)

// ChainAPI implements Chain to provide access to the chain-related RPC methods
type ChainAPI struct {
	client *RPCClient
}

// GetBlock gets a block by height
func (c *ChainAPI) GetBlock(height uint64) (*types.ResultBlock, error) {

	resp, statusCode, err := c.client.call("chain_getBlock", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = types.ResultBlock{ResultBlock: &core_types.ResultBlock{}}
	err = util.DecodeWithJSON(resp["result"], r.ResultBlock)
	if err != nil {
		pp.Println(resp["result"])
		return nil, err
	}

	return &r, nil
}
