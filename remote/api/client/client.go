package client

import (
	"fmt"
	"path"
	"strings"

	"github.com/imroc/req"
	"gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/state"
)

// Client describes methods for accessing REST API endpoints
type Client interface {
	SendTxPayload(data map[string]interface{}) (*api.SendTxPayloadResponse, error)
	GetAccountNonce(address string, blockHeight ...uint64) (*api.GetAccountNonceResponse, error)
	GetAccount(address string, blockHeight ...uint64) (*state.Account, error)
	GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error)
	PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error)
	GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*api.GetAccountNonceResponse, error)
	GetPushKey(pushKeyID string, blockHeight ...uint64) (*state.PushKey, error)
}

// RequestFunc describes the function for making http requests
type RequestFunc func(endpoint string, params map[string]interface{}) (*req.Resp, error)

// M is actually a map[string]interface{}
type M map[string]interface{}

func joinURL(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}

// ClientV1 is a REST API client
type ClientV1 struct {
	apiRoot string
	get     RequestFunc
	post    RequestFunc
}

// NewClient creates an instance of ClientV1;
//
// ARGS:
// apiRoot is the URL path to the API server
func NewClient(apiRoot string) *ClientV1 {
	c := &ClientV1{apiRoot: apiRoot}
	c.get = c.GetCall
	c.post = c.PostCall
	return c
}

// GetCall makes a get call to the endpoint
func (c *ClientV1) GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error) {
	url := joinURL(c.apiRoot, endpoint)
	return req.Get(url, req.QueryParam(params))
}

// PostCall makes a get call to the endpoint
func (c *ClientV1) PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error) {
	url := joinURL(c.apiRoot, endpoint)
	return req.Post(url, req.BodyJSON(body))
}
