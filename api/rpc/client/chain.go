package client

import (
	"github.com/make-os/lobe/api/types"
)

// ChainAPI implements Chain to provide access to the chain-related RPC methods
type ChainAPI struct {
	client Client
}

// Get gets the account that owns a push key
func (c *ChainAPI) Get(id string, blockHeight ...uint64) (*types.GetAccountResponse, error) {
	return nil, nil
}
