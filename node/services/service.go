package services

import (
	"net"
	"net/url"

	"github.com/pkg/errors"
	tmconfig "github.com/tendermint/tendermint/config"
)

// Service provides operation that access external logic not
// directly offered by any of the packages of the project.
// For instance, access to tendermint RPC is provided.
type Service interface {
	GetBlock(height int64) (map[string]interface{}, error)
}

// NodeService implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type NodeService struct {
	tmrpc *TMRPC
}

// New creates an instance of NodeService
func New(tmAddress string) *NodeService {
	return &NodeService{
		tmrpc: newTMRPC(tmAddress),
	}
}

// NewFromConfig creates and instance of NodeService
// with addressed sourced from tendermint config.
func NewFromConfig(tmCfg *tmconfig.Config) (Service, error) {
	addr, err := url.Parse(tmCfg.RPC.ListenAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse RPC address")
	}
	return New(net.JoinHostPort(addr.Hostname(), addr.Port())), nil
}
