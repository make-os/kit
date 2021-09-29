package api

import (
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
)

// Manager provides RPC service management functionality.
type Manager struct {
}

// NewRPCManagerAPI creates an instance of Manager
func NewRPCManagerAPI() *Manager {
	return &Manager{}
}

// echo returns any parameter sent in the request
func (l *Manager) echo(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{"data": params})
}

// APIs returns all API handlers
func (l *Manager) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:      "echo",
			Namespace: constants.NamespaceRPC,
			Desc:      "Returns echos back any parameter sent in the request",
			Func:      l.echo,
		},
	}
}
