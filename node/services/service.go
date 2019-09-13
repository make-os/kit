package services

import (
	"github.com/makeos/mosdef/node/tmrpc"
	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

const (
	// SrvNameCoinSend indicates service name for sending the native coins
	SrvNameCoinSend = "coin.send"
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
	default:
		return nil, types.ErrServiceMethodUnknown
	}
}

// sendCoin processes a types.TxTypeCoin transaction.
// Expects a signed transaction.
func (s *Service) sendCoin(data interface{}) (interface{}, error) {

	tx, ok := data.(*types.Transaction)
	if !ok {
		return nil, types.ErrArgDecode("types.Transaction", 0)
	}

	// Validate the transaction (syntax)
	if err := validators.ValidateTxSyntax(tx, -1); err != nil {
		return nil, err
	}

	// Validate the transaction (consistency)
	if err := validators.ValidateTxConsistency(tx, -1); err != nil {
		return nil, err
	}

	// Send the transaction to tendermint for processing
	txHash, err := s.tmrpc.SendTx(tx.Bytes())
	if err != nil {
		return nil, err
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": txHash.HexStr(),
	}), nil
}
