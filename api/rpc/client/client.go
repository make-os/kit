package client

//go:generate mockgen -destination=../mocks/mock_client.go -package=mocks github.com/ellcrys/partnertracker/rpcclient Client

import (
	"bytes"
	encJson "encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/rpc/v2/json"
)

// Timeout is the max duration for connection and read attempt
const Timeout = time.Duration(15 * time.Second)

// Client represents a JSON-RPC client
type Client interface {
	Call(method string, params interface{}) (interface{}, error)
	New(opts *Options) Client
	GetOptions() *Options
}

// Options describes the options used to
// configure the client
type Options struct {
	Host     string
	Port     int
	HTTPS    bool
	User     string
	Password string
}

// URL returns a fully formed url to
// use for making requests
func (o *Options) URL() string {
	protocol := "http://"
	if o.HTTPS {
		protocol = "https://"
	}
	return protocol + net.JoinHostPort(o.Host, strconv.Itoa(o.Port))
}

// RPCClient provides the ability to interact with a JSON-RPC 2.0 service
type RPCClient struct {
	c    *http.Client
	opts *Options
}

// Error represents a custom JSON-RPCerror
type Error struct {
	Data map[string]interface{} `json:"data"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v", e)
}

// NewClient creates an instance of Client
func NewClient(opts *Options) *RPCClient {

	if opts == nil {
		opts = &Options{}
	}

	if opts.Host == "" {
		panic("options.host is required")
	}

	if opts.Port == 0 {
		panic("options.port is required")
	}

	return &RPCClient{
		c:    new(http.Client),
		opts: opts,
	}
}

// GetOptions returns the client's option
func (c *RPCClient) GetOptions() *Options {
	return c.opts
}

// Call calls a method on the RPCClient service.
func (c *RPCClient) Call(method string, params interface{}) (interface{}, error) {

	if c.c == nil {
		return nil, fmt.Errorf("http client and options not set")
	}

	var request = map[string]interface{}{
		"method":  method,
		"params":  params,
		"id":      uint64(rand.Int63()),
		"jsonrpc": "2.0",
	}

	msg, err := encJson.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.opts.URL(), bytes.NewBuffer(msg))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if c.opts.User != "" && c.opts.Password != "" {
		req.SetBasicAuth(c.opts.User, c.opts.Password)
	}

	c.c.Timeout = Timeout
	resp, err := c.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call method: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("request unsuccessful: Status code: %d. Body: %s",
			resp.StatusCode, string(body))
	}

	var m interface{}
	err = json.DecodeClientResponse(resp.Body, &m)
	if err != nil {
		if e, ok := err.(*json.Error); ok {
			return nil, &Error{Data: e.Data.(map[string]interface{})}
		}
		return nil, err
	}

	return m, nil
}
