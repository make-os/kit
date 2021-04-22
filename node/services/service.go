package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/txns"
	"github.com/tendermint/tendermint/rpc/client/http"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

// Service provides operation that access external logic not
// directly offered by any of the packages of the project.
// For instance, access to tendermint RPC is provided.
type Service interface {
	GetBlock(ctx context.Context, height *int64) (*core_types.ResultBlock, error)
	IsSyncing(ctx context.Context) (bool, error)
	NetInfo(ctx context.Context) (*core_types.ResultNetInfo, error)
	GetTx(ctx context.Context, hash []byte, proof bool) (types.BaseTx, *tmtypes.TxProof, error)
}

// NodeService implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type NodeService struct {
	client *http.HTTP
}

// New creates an instance of NodeService.
// The function panics if rpcAddr is an invalid address.
func New(rpcAddr string) *NodeService {
	c, err := http.New(rpcAddr, "/websocket")
	if err != nil {
		panic(c)
	}
	return &NodeService{
		client: c,
	}
}

// GetBlock fetches a block at the given height
func (s *NodeService) GetBlock(ctx context.Context, height *int64) (*core_types.ResultBlock, error) {
	return s.client.Block(ctx, height)
}

// IsSyncing checks whether the node has caught up with the network
func (s *NodeService) IsSyncing(ctx context.Context) (bool, error) {
	status, err := s.client.Status(ctx)
	if err != nil {
		return false, err
	}
	return status.SyncInfo.CatchingUp, nil
}

// NetInfo returns network information
func (s *NodeService) NetInfo(ctx context.Context) (*core_types.ResultNetInfo, error) {
	ni, err := s.client.NetInfo(ctx)
	if err != nil {
		return nil, err
	}
	return ni, nil
}

// GetTx gets a transaction by hash
func (s *NodeService) GetTx(ctx context.Context, hash []byte, proof bool) (types.BaseTx, *tmtypes.TxProof, error) {
	res, err := s.client.Tx(ctx, hash, proof)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil, types.ErrTxNotFound
		}
		return nil, nil, err
	}
	tx, err := txns.DecodeTx(res.Tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode tx")
	}
	return tx, &res.Proof, nil
}
