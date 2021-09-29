package api

import (
	types2 "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
)

// ChainAPI provides APIs for accessing blockchain information
type ChainAPI struct {
	mods *types2.Modules
}

// NewChainAPI creates an instance of ChainAPI
func NewChainAPI(mods *types2.Modules) *ChainAPI {
	return &ChainAPI{mods}
}

// getBlock gets full block data at the given height
func (c *ChainAPI) getBlock(params interface{}) (resp *rpc.Response) {
	return rpc.Success(c.mods.Chain.GetBlock(cast.ToString(params)))
}

// getHeight gets the current blockchain height
func (c *ChainAPI) getHeight(interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"height": c.mods.Chain.GetCurHeight(),
	})
}

// getBlockInfo gets summarized block data at the given height
func (c *ChainAPI) getBlockInfo(params interface{}) (resp *rpc.Response) {
	return rpc.Success(c.mods.Chain.GetBlockInfo(cast.ToString(params)))
}

// getValidators gets validators of a given block
func (c *ChainAPI) getValidators(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"validators": c.mods.Chain.GetValidators(cast.ToString(params)),
	})
}

// isSync checks whether the node is syncing with peers
func (c *ChainAPI) isSyncing(_ interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"syncing": c.mods.Chain.IsSyncing(),
	})
}

// APIs returns all API handlers
func (c *ChainAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:      "getBlock",
			Namespace: constants.NamespaceNode,
			Desc:      "Get a block at a given chain height",
			Func:      c.getBlock,
		},
		{
			Name:      "getHeight",
			Namespace: constants.NamespaceNode,
			Desc:      "Get the current height of the blockchain",
			Func:      c.getHeight,
		},
		{
			Name:      "getBlockInfo",
			Namespace: constants.NamespaceNode,
			Desc:      "Get summarized block data at the given height",
			Func:      c.getBlockInfo,
		},
		{
			Name:      "getValidators",
			Namespace: constants.NamespaceNode,
			Desc:      "Get validators at a given height",
			Func:      c.getValidators,
		},
		{
			Name:      "isSyncing",
			Namespace: constants.NamespaceNode,
			Desc:      "Get validators at a given height",
			Func:      c.isSyncing,
		},
	}
}
