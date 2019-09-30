package services

import (
	"strconv"

	"github.com/stretchr/objx"
)

// GetBlock fetches a block at the given height
func (s *Service) GetBlock(height int64) (map[string]interface{}, error) {

	blockInfo, err := s.tmrpc.GetBlock(height)
	if err != nil {
		return nil, err
	}

	return blockInfo, nil
}

// GetCurrentHeight fetches the current block height
func (s *Service) GetCurrentHeight() (int64, error) {

	blockInfo, err := s.tmrpc.GetBlock(0)
	if err != nil {
		return 0, err
	}

	heightStr := objx.Map(blockInfo).Get("result.block.header.height").Str()
	heightInt, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return heightInt, nil
}
