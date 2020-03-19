package rpc

import (
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
)

// RPCManagerAPI provides RPC service management functionality.
type RPCManagerAPI struct {
	server *rpc.Server
}

// NewRPCManagerAPI creates an instance of RPCManagerAPI
func NewRPCManagerAPI(srv *rpc.Server) *RPCManagerAPI {
	return &RPCManagerAPI{srv}
}

// echo returns any parameter sent in the request
// Body:
// - params <any>: Arbitrary parameter
// Response <map>:
// - data <any>: The params passed in the request
func (l *RPCManagerAPI) echo(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{"data": params})
}

// APIs returns all API handlers
func (l *RPCManagerAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"echo": {
			Namespace:   types.NamespaceRPC,
			Description: "Returns echos back any parameter sent in the request",
			Func:        l.echo,
		},
	}
}
