package rpc

import (
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

// Manager provides RPC service management functionality.
type Manager struct {
	server *rpc.Server
}

// NewRPCManagerAPI creates an instance of Manager
func NewRPCManagerAPI(srv *rpc.Server) *Manager {
	return &Manager{srv}
}

// echo returns any parameter sent in the request
// Body:
// - params <any>: Arbitrary parameter
// Response <map>:
// - data <any>: The params passed in the request
func (l *Manager) echo(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{"data": params})
}

// APIs returns all API handlers
func (l *Manager) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"echo": {
			Namespace:   constants.NamespaceRPC,
			Description: "Returns echos back any parameter sent in the request",
			Func:        l.echo,
		},
	}
}
