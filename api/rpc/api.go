package rpc

import (
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/modules"
)

type TestCase struct {
	params     interface{}
	body       string
	statusCode int
	mocker     func(tc *TestCase)
	result     map[string]interface{}
	err        *rpc.Err
}

// APIs returns all API handlers
func APIs(modulesAgg modules.ModuleHub, rpcServer *rpc.Server) rpc.APISet {

	mods := modulesAgg.GetModules()
	var apiSets = []rpc.APISet{
		NewAccountAPI(mods).APIs(),
		NewPushKeyAPI(mods).APIs(),
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
