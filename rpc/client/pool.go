package client

import (
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
)

// PoolAPI implements Pool to provide access to the DHT network
type PoolAPI struct {
	c *RPCClient
}

// GetSize returns size information of the mempool
func (d *PoolAPI) GetSize() (*api.ResultPoolSize, error) {
	resp, statusCode, err := d.c.call("pool_getSize", nil)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r api.ResultPoolSize
	if err := util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetPushPoolSize returns size information of the mempool
func (d *PoolAPI) GetPushPoolSize() (int, error) {
	resp, statusCode, err := d.c.call("pool_getPushPoolSize", nil)
	if err != nil {
		return 0, makeStatusErrorFromCallErr(statusCode, err)
	}
	return cast.ToInt(resp["size"]), nil
}
