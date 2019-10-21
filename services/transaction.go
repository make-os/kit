package services

import (
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// SendTx sends a types.TxTypeExecCoinTransfer transaction to the network.
// CONTRACT: Expects a signed transaction.
func (s *Service) SendTx(tx *types.Transaction) (util.Hash, error) {
	return s.txRec.AddTx(tx)
}
