package rpc

import (
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
)

// APIs returns all API handlers
func APIs(modulesHub types.ModulesHub, rpcServer *rpc.RPCServer) rpc.APISet {

	// Create a new module instances for RPC environment.
	modules := modulesHub.GetModules()

	// Collect APIs
	var apiSets = []rpc.APISet{
		NewAccountAPI(modules).APIs(),
		NewPushKeyAPI(modules).APIs(),
		NewLocalAccountAPI(modules).APIs(),
		NewTransactionAPI(modules).APIs(),
		NewRPCManagerAPI(rpcServer).APIs(),
		NewRepoAPI(modules).APIs(),
	}

	var mainSet = []rpc.APIInfo{}
	for _, set := range apiSets {
		for _, v := range set {
			mainSet = append(mainSet, v)
		}
	}

	return mainSet
}
