package services

import (
	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// SendCoin sends a types.TxTypeCoin transaction to the network.
// CONTRACT: Expects a signed transaction.
func (s *Service) SendCoin(tx *types.Transaction) (util.Hash, error) {
	var hash util.Hash

	// Validate the transaction
	if err := validators.ValidateTx(tx, -1, s.logic); err != nil {
		return hash, err
	}

	return s.txRec.AddTx(tx)
}
