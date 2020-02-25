package services

// GetBlock fetches a block at the given height
func (s *NodeService) GetBlock(height int64) (map[string]interface{}, error) {
	blockInfo, err := s.tmrpc.getBlock(height)
	if err != nil {
		return nil, err
	}
	return blockInfo, nil
}
