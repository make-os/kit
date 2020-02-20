package logic

import (
	"gitlab.com/makeos/mosdef/logic/api"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
)

// APIs returns all API handlers
func (l *Logic) APIs() jsonrpc.APISet {

	var apiSets = []jsonrpc.APISet{
		api.NewAccountAPI(l).APIs(),
		api.NewGPGAPI(l).APIs(),
	}

	var mainSet = make(map[string]jsonrpc.APIInfo)
	for _, set := range apiSets {
		for k, v := range set {
			mainSet[k] = v
		}
	}

	return mainSet
}
