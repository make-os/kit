package client

import (
	"fmt"
	"path"
	"strings"

	"github.com/imroc/req"
)

// Client describes methods for accessing REST API endpoints
type Client interface {

	// GetCall makes a get call to the endpoint.
	// Responses not within 200-299 are considered an error.
	GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error)

	// PostCall makes a get call to the endpoint.
	// Responses not within 200-299 are considered an error.
	PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error)

	// PushKey exposes methods for managing push keys
	PushKey() PushKey

	// Repo exposes methods for managing repositories
	Repo() Repo

	// Tx exposes methods for creating and accessing the transactions
	Tx() Tx

	// User exposes methods for accessing user information
	User() User
}

// RequestFunc describes the function for making http requests
type RequestFunc func(endpoint string, params map[string]interface{}) (*req.Resp, error)

// M is actually a map[string]interface{}
type M map[string]interface{}

func joinURL(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}

// RemoteClient is a REST API client
type RemoteClient struct {
	apiRoot string
	get     RequestFunc
	post    RequestFunc
}

// NewClient creates an instance of RemoteClient;
//
// ARGS:
//  - apiRoot is the URL path to the API server
func NewClient(apiRoot string) *RemoteClient {
	c := &RemoteClient{apiRoot: apiRoot}
	c.get = c.GetCall
	c.post = c.PostCall
	return c
}

// PushKey exposes methods for managing push keys
func (c *RemoteClient) PushKey() PushKey {
	return &PushKeyAPI{c: c}
}

// Repo exposes methods for managing repositories
func (c *RemoteClient) Repo() Repo {
	return &RepoAPI{c: c}
}

// Tx exposes methods for creating and accessing the transactions
func (c *RemoteClient) Tx() Tx {
	return &TxAPI{c: c}
}

// User exposes methods for accessing user information
func (c *RemoteClient) User() User {
	return &UserAPI{c: c}
}

// GetCall makes a get call to the endpoint.
// Responses not within 200-299 are considered an error.
func (c *RemoteClient) GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error) {
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
func (c *RemoteClient) PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error) {
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

// V1Path creates a REST API v1 path
func V1Path(ns, method string) string {
	return fmt.Sprintf("/v1/%s/%s", ns, method)
}
