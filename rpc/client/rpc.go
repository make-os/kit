package client

import (
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
)

// RPCAPI provides access to the rpc server-related methods
type RPCAPI struct {
	c *RPCClient
}

// GetMethods gets all methods supported by the RPC server
func (c *RPCAPI) GetMethods() ([]rpc.MethodInfo, error) {
	resp, statusCode, err := c.c.call("rpc_methods", nil)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r []rpc.MethodInfo
	if err := util.DecodeMap(resp["methods"], &r); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}
