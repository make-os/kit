package services

import (
	"github.com/makeos/mosdef/node/tmrpc"
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
