package services

import (
	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/tmrpc"
	"gitlab.com/makeos/mosdef/types/core"
)

// Service implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type Service struct {
	tmrpc *tmrpc.TMRPC
	logic core.Logic
	txRec *mempool.Reactor
}

// New creates an instance of Service
func New(tmrpc *tmrpc.TMRPC, logic core.Logic, txRec *mempool.Reactor) *Service {
	return &Service{
		tmrpc: tmrpc,
		logic: logic,
		txRec: txRec,
	}
}
