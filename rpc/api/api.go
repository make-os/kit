package api

import (
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
)

// APIs returns all API handlers
func APIs(modulesHub types.ModulesHub) rpc.APISet {

	// Create a new module instances for RPC environment.
	modules := modulesHub.GetModules()

	// Collect APIs
	var apiSets = []rpc.APISet{
		NewUserAPI(modules).APIs(),
		NewPushKeyAPI(modules).APIs(),
		NewTransactionAPI(modules).APIs(),
		NewRPCManagerAPI().APIs(),
		NewRepoAPI(modules).APIs(),
		NewChainAPI(modules).APIs(),
		NewDHTAPI(modules).APIs(),
		NewNamespaceAPI(modules).APIs(),
		NewPoolAPI(modules).APIs(),
		NewTicketAPI(modules).APIs(),
	}

	var mainSet = []rpc.MethodInfo{}
	for _, set := range apiSets {
		for _, v := range set {
			mainSet = append(mainSet, v)
		}
	}

	return mainSet
}
