package rpc

import (
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/rpc/jsonrpc"
	"gitlab.com/makeos/mosdef/types"
)

// RPCManagerAPI provides RPC methods for various local account management functionality.
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
// Response:
// - resp <any> - Returns the inputted params
func (l *RPCManagerAPI) echo(params interface{}) (resp *jsonrpc.Response) {
	return jsonrpc.Success(params)
}

// APIs returns all API handlers
func (l *RPCManagerAPI) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"echo": {
			Namespace:   types.NamespaceRPC,
			Description: "Returns echos back any parameter sent in the request",
			Func:        l.echo,
		},
	}
}
