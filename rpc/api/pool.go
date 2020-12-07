package api

import (
	modtypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
)

// PoolAPI provides APIs for accessing the mempool and pushpool
type PoolAPI struct {
	mods *modtypes.Modules
}

// NewPoolAPI creates an instance of PoolAPI
func NewPoolAPI(mods *modtypes.Modules) *PoolAPI {
	return &PoolAPI{mods}
}

// getSize returns mempool size information
func (c *PoolAPI) getSize(params interface{}) (resp *rpc.Response) {
	return rpc.Success(c.mods.Pool.GetSize())
}

// getTop returns transactions from the mempool beginning from the head
func (c *PoolAPI) getTop(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"txs": c.mods.Pool.GetTop(cast.ToInt(params)),
	})
}

// getPushPoolSize returns the size of the pushpool
func (c *PoolAPI) getPushPoolSize(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{"size": c.mods.Pool.GetPushPoolSize()})
}

// APIs returns all API handlers
func (c *PoolAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:        "getSize",
			Namespace:   constants.NamespacePool,
			Description: "Get mempool size information",
			Func:        c.getSize,
		},
		{
			Name:        "getTop",
			Namespace:   constants.NamespacePool,
			Description: "Get top transactions from the mempool",
			Func:        c.getTop,
		},
		{
			Name:        "getPushPoolSize",
			Namespace:   constants.NamespacePool,
			Description: "Get the size of the pushpool",
			Func:        c.getPushPoolSize,
		},
	}
}
