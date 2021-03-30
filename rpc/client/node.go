package client

import (
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/spf13/cast"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
)

// ChainAPI implements Node to provide access to the chain-related RPC methods
type ChainAPI struct {
	c *RPCClient
}

// GetBlock gets a block by height
func (c *ChainAPI) GetBlock(height uint64) (*api.ResultBlock, error) {
	resp, statusCode, err := c.c.call("node_getBlock", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = api.ResultBlock{ResultBlock: &core_types.ResultBlock{}}
	if err = util.DecodeWithJSON(resp, r.ResultBlock); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetHeight returns the height of the blockchain
func (c *ChainAPI) GetHeight() (uint64, error) {
	resp, statusCode, err := c.c.call("node_getHeight", nil)
	if err != nil {
		return 0, makeStatusErrorFromCallErr(statusCode, err)
	}
	return cast.ToUint64(resp["height"]), nil
}

// GetBlockInfo gets a summarized block data for the given height
func (c *ChainAPI) GetBlockInfo(height uint64) (*api.ResultBlockInfo, error) {
	resp, statusCode, err := c.c.call("node_getBlockInfo", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = api.ResultBlockInfo{BlockInfo: &state.BlockInfo{}}
	if err = util.DecodeWithJSON(resp, r.BlockInfo); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetValidators gets validators at a given block height
func (c *ChainAPI) GetValidators(height uint64) ([]*api.ResultValidator, error) {
	resp, statusCode, err := c.c.call("node_getValidators", height)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = []*api.ResultValidator{}
	if err = util.DecodeMap(resp["validators"], &r); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// IsSyncing checks whether the node is synchronizing with peers
func (c *ChainAPI) IsSyncing() (bool, error) {
	resp, statusCode, err := c.c.call("node_isSyncing", nil)
	if err != nil {
		return false, makeStatusErrorFromCallErr(statusCode, err)
	}
	return cast.ToBool(resp["syncing"]), nil
}
