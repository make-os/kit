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
	IsSyncing() (bool, error)
	NetInfo() (*core_types.ResultNetInfo, error)
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

// GetBlock fetches a block at the given height
func (s *NodeService) GetBlock(height int64) (*core_types.ResultBlock, error) {
	var h = &height
	if *h == 0 {
		h = nil
	}
	return s.tmrpc.Block(h)
}

// IsSyncing checks whether the node has caught up with the rest of its connected peers
func (s *NodeService) IsSyncing() (bool, error) {
	status, err := s.tmrpc.Status()
	if err != nil {
		return false, err
	}
	return !status.SyncInfo.CatchingUp, nil
}

// NetInfo returns network information
func (s *NodeService) NetInfo() (*core_types.ResultNetInfo, error) {
	ni, err := s.tmrpc.NetInfo()
	if err != nil {
		return nil, err
	}
	return ni, nil
}
