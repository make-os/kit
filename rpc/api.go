package rpc

import (
	"github.com/makeos/mosdef/rpc/jsonrpc"
	"github.com/makeos/mosdef/types"
)

func (s *Server) apiRPCStop(params interface{}) *jsonrpc.Response {
	s.Stop()
	return jsonrpc.Success(true)
}

func (s *Server) apiRPCEcho(params interface{}) *jsonrpc.Response {
	return jsonrpc.Success(params)
}

// APIs returns all API handlers
func (s *Server) APIs() jsonrpc.APISet {
	return map[string]jsonrpc.APIInfo{
		"echo": {
			Namespace:   types.NamespaceRPC,
			Description: "Returns the parameter passed to it",
			Func:        s.apiRPCEcho,
		},
	}
}
