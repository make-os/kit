package services

import (
	types2 "gitlab.com/makeos/mosdef/logic/types"
	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/tmrpc"
)

// Service implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type Service struct {
	tmrpc *tmrpc.TMRPC
	logic types2.Logic
	txRec *mempool.Reactor
}

// New creates an instance of Service
func New(tmrpc *tmrpc.TMRPC, logic types2.Logic, txRec *mempool.Reactor) *Service {
	return &Service{
		tmrpc: tmrpc,
		logic: logic,
		txRec: txRec,
	}
}
