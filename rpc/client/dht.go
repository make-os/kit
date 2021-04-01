package client

import (
	"encoding/base64"

	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/spf13/cast"
)

// DHTAPI implements DHT to provide access to the DHT network
type DHTAPI struct {
	c *RPCClient
}

// GetPeers returns node IDs of connected peers
func (d *DHTAPI) GetPeers() ([]string, error) {
	resp, statusCode, err := d.c.call("dht_getPeers", nil)
	if err != nil {
		return nil, makeReqErrFromCallErr(statusCode, err)
	}
	return cast.ToStringSlice(resp["peers"]), nil
}

// GetProviders returns providers of the given key
func (d *DHTAPI) GetProviders(key string) ([]*api.ResultDHTProvider, error) {
	resp, statusCode, err := d.c.call("dht_getProviders", key)
	if err != nil {
		return nil, makeReqErrFromCallErr(statusCode, err)
	}

	var r = []*api.ResultDHTProvider{}
	if err = util.DecodeMap(resp["providers"], &r); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// Announce announces the given key to the network
func (d *DHTAPI) Announce(key string) error {
	_, statusCode, err := d.c.call("dht_announce", key)
	if err != nil {
		return makeReqErrFromCallErr(statusCode, err)
	}
	return nil
}

// GetRepoObjectProviders returns providers for the given repository object hash
func (d *DHTAPI) GetRepoObjectProviders(hash string) ([]*api.ResultDHTProvider, error) {
	resp, statusCode, err := d.c.call("dht_getRepoObjectProviders", hash)
	if err != nil {
		return nil, makeReqErrFromCallErr(statusCode, err)
	}

	var r = []*api.ResultDHTProvider{}
	if err = util.DecodeMap(resp["providers"], &r); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// Store stores a value under the given key on the DHT
func (d *DHTAPI) Store(key, value string) error {
	_, statusCode, err := d.c.call("dht_store", util.Map{"key": key, "value": value})
	if err != nil {
		return makeReqErrFromCallErr(statusCode, err)
	}
	return nil
}

// Lookup finds a value stored under the given key
func (d *DHTAPI) Lookup(key string) (string, error) {
	resp, statusCode, err := d.c.call("dht_lookup", key)
	if err != nil {
		return "", makeReqErrFromCallErr(statusCode, err)
	}

	bz, err := base64.StdEncoding.DecodeString(cast.ToString(resp["value"]))
	if err != nil {
		return "", errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return string(bz), nil
}
