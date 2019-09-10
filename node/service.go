package node

import (
	"github.com/k0kubun/pp"
	"github.com/makeos/mosdef/types"
)

const (
	// SrvNameCoinSend indicates service name for sending the native coins
	SrvNameCoinSend = "coin.send"
)

// Service implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type Service struct {
}

// NewService creates an instance of Service
func NewService() *Service {
	return &Service{}
}

// Do collects request for the execution of a service
func (s *Service) Do(method string, params ...interface{}) (interface{}, error) {
	switch method {
	case SrvNameCoinSend:
		return s.sendCoin(params)
	default:
		return nil, types.ErrServiceMethodUnknown
	}
}

func (s *Service) sendCoin(params ...interface{}) (interface{}, error) {
	pp.Println("Send It Now", params)
	return nil, nil
}
