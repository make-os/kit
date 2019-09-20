package services

import (
	"github.com/makeos/mosdef/node/tmrpc"
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
