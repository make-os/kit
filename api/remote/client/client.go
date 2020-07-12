package client

import (
	"fmt"
	"path"
	"strings"

	"github.com/imroc/req"
	"gitlab.com/makeos/mosdef/api/types"
)

// Client describes methods for accessing REST API endpoints
type Client interface {
	GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error)
	PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error)
	SendTxPayload(data map[string]interface{}) (*types.HashResponse, error)
	GetAccountNonce(address string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error)
	GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, error)
	GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error)
	GetPushKey(pushKeyID string, blockHeight ...uint64) (*types.GetPushKeyResponse, error)
	RegisterPushKey(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error)
	CreateRepo(body *types.CreateRepoBody) (*types.CreateRepoResponse, error)
	GetRepo(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error)
	AddRepoContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error)
	SendCoin(body *types.SendCoinBody) (*types.HashResponse, error)
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

// GetCall makes a get call to the endpoint.
// Responses not within 200-299 are considered an error.
func (c *ClientV1) GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error) {
	url := joinURL(c.apiRoot, endpoint)
	resp, err := req.Get(url, req.QueryParam(params))
	if err != nil {
		return nil, err
	}
	if resp.Response().StatusCode >= 200 && resp.Response().StatusCode <= 299 {
		return resp, nil
	}
	return resp, fmt.Errorf(resp.String())
}

// PostCall makes a get call to the endpoint.
// Responses not within 200-299 are considered an error.
func (c *ClientV1) PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error) {
	url := joinURL(c.apiRoot, endpoint)
	resp, err := req.Post(url, req.BodyJSON(body))
	if err != nil {
		return nil, err
	}
	if resp.Response().StatusCode >= 200 && resp.Response().StatusCode <= 299 {
		return resp, nil
	}
	return resp, fmt.Errorf(resp.String())
}
