package services

import (
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// SendCoin sends a types.TxTypeTransferCoin transaction to the network.
// CONTRACT: Expects a signed transaction.
func (s *Service) SendCoin(tx *types.Transaction) (util.Hash, error) {
	return s.txRec.AddTx(tx)
}
