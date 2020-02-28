package rpc

import (
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
)

// APIs returns all API handlers
func APIs(modulesAgg types.ModulesAggregator, rpcServer *rpc.Server) rpc.APISet {

	mods := modulesAgg.GetModules().(*modules.Modules)
	var apiSets = []rpc.APISet{
		NewAccountAPI(mods).APIs(),
		NewGPGAPI(mods).APIs(),
		NewLocalAccountAPI(mods).APIs(),
		NewTransactionAPI(mods).APIs(),
		NewRPCManagerAPI(rpcServer).APIs(),
	}

	var mainSet = make(map[string]rpc.APIInfo)
	for _, set := range apiSets {
		for k, v := range set {
			mainSet[k] = v
		}
	}

	return mainSet
}
