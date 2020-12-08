package services

import (
	"context"

	"github.com/tendermint/tendermint/rpc/client/http"
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
	tmrpc *http.HTTP
}

// New creates an instance of NodeService.
// The function panics if rpcAddr is an invalid address.
func New(rpcAddr string) *NodeService {

	c, err := http.New(rpcAddr, "/websocket")
	if err != nil {
		panic(c)
	}

	return &NodeService{
		tmrpc: c,
	}
}

// GetBlock fetches a block at the given height
func (s *NodeService) GetBlock(height int64) (*core_types.ResultBlock, error) {
	var h = &height
	if *h == 0 {
		h = nil
	}
	return s.tmrpc.Block(context.Background(), h)
}

// IsSyncing checks whether the node has caught up with the rest of its connected peers
func (s *NodeService) IsSyncing() (bool, error) {
	status, err := s.tmrpc.Status(context.Background())
	if err != nil {
		return false, err
	}
	return !status.SyncInfo.CatchingUp, nil
}

// NetInfo returns network information
func (s *NodeService) NetInfo() (*core_types.ResultNetInfo, error) {
	ni, err := s.tmrpc.NetInfo(context.Background())
	if err != nil {
		return nil, err
	}
	return ni, nil
}
