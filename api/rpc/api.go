package rpc

import (
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
)

// APIs returns all API handlers
func APIs(modulesHub types.ModulesHub, rpcServer *rpc.RPCServer) rpc.APISet {

	// Create a new module instances for RPC environment.
	modules := modulesHub.GetModules()

	// Collect APIs
	var apiSets = []rpc.APISet{
		NewAccountAPI(modules).APIs(),
		NewPushKeyAPI(modules).APIs(),
		NewTransactionAPI(modules).APIs(),
		NewRPCManagerAPI(rpcServer).APIs(),
		NewRepoAPI(modules).APIs(),
		NewChainAPI(modules).APIs(),
		NewDHTAPI(modules).APIs(),
		NewNamespaceAPI(modules).APIs(),
		NewPoolAPI(modules).APIs(),
		NewTicketAPI(modules).APIs(),
	}

	var mainSet = []rpc.APIInfo{}
	for _, set := range apiSets {
		for _, v := range set {
			mainSet = append(mainSet, v)
		}
	}

	return mainSet
}
