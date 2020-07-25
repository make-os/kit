package rpc

import (
	"gitlab.com/makeos/lobe/rpc"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/util"
)

// Manager provides RPC service management functionality.
type Manager struct {
	server *rpc.RPCServer
}

// NewRPCManagerAPI creates an instance of Manager
func NewRPCManagerAPI(srv *rpc.RPCServer) *Manager {
	return &Manager{srv}
}

// echo returns any parameter sent in the request
func (l *Manager) echo(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{"data": params})
}

// APIs returns all API handlers
func (l *Manager) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{
			Name:        "echo",
			Namespace:   constants.NamespaceRPC,
			Description: "Returns echos back any parameter sent in the request",
			Func:        l.echo,
		},
	}
}
