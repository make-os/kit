package services

import (
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
)

// SendTx sends a types.TxTypeCoinTransfer transaction to the network.
// CONTRACT: Expects a signed transaction.
func (s *Service) SendTx(tx types.BaseTx) (util.Bytes32, error) {
	return s.txRec.AddTx(tx)
}
