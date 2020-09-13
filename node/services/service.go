package services

import (
	"github.com/tendermint/tendermint/rpc/client"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
)

// Service provides operation that access external logic not
// directly offered by any of the packages of the project.
// For instance, access to tendermint RPC is provided.
type Service interface {
	GetBlock(height int64) (*core_types.ResultBlock, error)
}

// NodeService implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type NodeService struct {
	tmrpc *client.HTTP
}

// New creates an instance of NodeService
func New(rpcAddr string) *NodeService {
	return &NodeService{
		tmrpc: client.NewHTTP(rpcAddr, "/websocket"),
	}
}
