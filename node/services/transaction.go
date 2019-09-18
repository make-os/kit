package services


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