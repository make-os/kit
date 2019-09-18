package services

import (
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/stretchr/objx"
)

// getCurrentHeight fetches a block at the given height
func (s *Service) getBlock(arg interface{}) (interface{}, error) {

	height, ok := arg.(int64)
	if !ok {
		return nil, types.ErrArgDecode("Int64", 0)
	}

	blockInfo, err := s.tmrpc.GetBlock(height)
	if err != nil {
		return nil, err
	}

	return util.EncodeForJS(blockInfo), nil
}

// getCurrentHeight fetches the current block height
func (s *Service) getCurrentHeight() (interface{}, error) {

	blockInfo, err := s.tmrpc.GetBlock(0)
	if err != nil {
		return nil, err
	}

	return util.EncodeForJS(map[string]interface{}{
		"height": objx.Map(blockInfo).Get("result.block.header.height").Str(),
	}), nil
}
