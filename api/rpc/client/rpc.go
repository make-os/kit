package client

import (
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/util"
)

// RPCAPI provides access to the rpc server-related methods
type RPCAPI struct {
	client *RPCClient
}

// GetMethods gets all methods supported by the RPC server
func (c *RPCAPI) GetMethods() (*types.ResultGetMethod, error) {
	resp, statusCode, err := c.client.call("rpc_methods", nil)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.ResultGetMethod
	if err := util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
