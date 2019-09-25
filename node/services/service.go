package services

import (
	"github.com/makeos/mosdef/node/reactors"
	"github.com/makeos/mosdef/node/tmrpc"
	"github.com/makeos/mosdef/types"
)

// Service implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type Service struct {
	tmrpc *tmrpc.TMRPC
	logic types.Logic
	txRec *reactors.TxReactor
}

// New creates an instance of Service
func New(tmrpc *tmrpc.TMRPC, logic types.Logic, txRec *reactors.TxReactor) *Service {
	return &Service{
		tmrpc: tmrpc,
		logic: logic,
		txRec: txRec,
	}
}
