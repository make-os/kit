package client

import (
	"github.com/make-os/lobe/types/api"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
)

// ChainAPI implements Chain to provide access to the chain-related RPC methods
type ChainAPI struct {
	client *RPCClient
}

// GetBlock gets a block by height
func (c *ChainAPI) GetBlock(height uint64) (*api.ResultBlock, error) {
	resp, statusCode, err := c.client.call("chain_getBlock", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = api.ResultBlock{ResultBlock: &core_types.ResultBlock{}}
	if err = util.DecodeWithJSON(resp, r.ResultBlock); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetHeight returns the height of the blockchain
func (c *ChainAPI) GetHeight() (uint64, error) {
	resp, statusCode, err := c.client.call("chain_getHeight", nil)
	if err != nil {
		return 0, makeStatusErrorFromCallErr(statusCode, err)
	}
	return cast.ToUint64(resp["height"]), nil
}

// GetBlockInfo gets a summarized block data for the given height
func (c *ChainAPI) GetBlockInfo(height uint64) (*api.ResultBlockInfo, error) {
	resp, statusCode, err := c.client.call("chain_getBlockInfo", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = api.ResultBlockInfo{BlockInfo: &state.BlockInfo{}}
	if err = util.DecodeWithJSON(resp, r.BlockInfo); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetValidators gets validators at a given block height
func (c *ChainAPI) GetValidators(height uint64) ([]*api.ResultValidator, error) {
	resp, statusCode, err := c.client.call("chain_getValidators", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = []*api.ResultValidator{}
	if err = util.DecodeMap(resp["validators"], &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}
