package rpc

import (
	"gitlab.com/makeos/mosdef/accountmgr"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
)

// APIs returns all API handlers
func APIs(
	modulesAgg types.ModulesAggregator,
	am *accountmgr.AccountManager,
	rpcServer *rpc.Server) jsonrpc.APISet {

	mods := modulesAgg.GetModules().(*modules.Modules)
	var apiSets = []jsonrpc.APISet{
		NewAccountAPI(mods).APIs(),
		NewGPGAPI(mods).APIs(),
		NewLocalAccountAPI(mods).APIs(),
		NewRPCManagerAPI(rpcServer).APIs(),
	}

	var mainSet = make(map[string]jsonrpc.APIInfo)
	for _, set := range apiSets {
		for k, v := range set {
			mainSet[k] = v
		}
	}

	return mainSet
}
