package services

import (
	"github.com/makeos/mosdef/node/tmrpc"
	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

const (
	// SrvNameCoinSend is the name of a service that sends the native coins
	SrvNameCoinSend = "coin.send"
	// SrvNameChainGetBlock is the name of a service that fetches blocks.
	SrvNameChainGetBlock = "chain.getBlock"
	// SrvNameGetCurBlockHeight is the name of a service that fetches blocks.
	SrvNameGetCurBlockHeight = "chain.currentHeight"
)

// Service implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type Service struct {
	tmrpc *tmrpc.TMRPC
}

// New creates an instance of Service
func New(tmrpc *tmrpc.TMRPC) *Service {
	return &Service{
		tmrpc: tmrpc,
	}
}

// Do collects request for the execution of a service
func (s *Service) Do(method string, params interface{}) (interface{}, error) {
	switch method {
	case SrvNameCoinSend:
		return s.sendCoin(params)
	case SrvNameChainGetBlock:
		return s.getBlock(params)
	case SrvNameGetCurBlockHeight:
		return s.getCurrentHeight()
	default:
		return nil, types.ErrServiceMethodUnknown
	}
}
