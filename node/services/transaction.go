package services

import (
	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// SendCoin processes a types.TxTypeCoin transaction.
// Expects a signed transaction.
func (s *Service) SendCoin(tx *types.Transaction) (util.Hash, error) {
	var hash util.Hash

	// Validate the transaction
	if err := validators.ValidateTx(tx, -1, s.logic); err != nil {
		return hash, err
	}

	// Send the transaction to tendermint for processing
	txHash, err := s.tmrpc.SendTx(tx.Bytes())
	if err != nil {
		return hash, err
	}

	return txHash, nil
}
