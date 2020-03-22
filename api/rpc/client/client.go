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
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// Timeout is the max duration for connection and read attempt
const (
	Timeout       = time.Duration(15 * time.Second)
	ErrCodeClient = "client_error"
)

// Client represents a JSON-RPC client
type Client interface {
	TxSendPayload(data map[string]interface{}) (*types.TxSendPayloadResponse, *util.StatusError)
	AccountGet(address string, blockHeight ...uint64) (*state.Account, *util.StatusError)
	PushKeyGetAccountOfOwner(id string, blockHeight ...uint64) (*state.Account, *util.StatusError)
	GetOptions() *Options
	Call(method string, params interface{}) (res util.Map, statusCode int, err error)
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

type callerFunc func(method string, params interface{}) (res util.Map, statusCode int, err error)

// RPCClient provides the ability to interact with a JSON-RPC 2.0 service
type RPCClient struct {
	c    *http.Client
	opts *Options
	call callerFunc
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

	client := &RPCClient{
		c:    new(http.Client),
		opts: opts,
	}
	client.call = client.Call

	return client
}

// GetOptions returns the client's option
func (c *RPCClient) GetOptions() *Options {
	return c.opts
}

// Call calls a method on the RPCClient service.
// Returns:
// res: JSON-RPC 2.0 success response
// statusCode: Server response code
// err: Client error or JSON-RPC 2.0 error response.
//      0 = Client error
func (c *RPCClient) Call(method string, params interface{}) (res util.Map, statusCode int, err error) {

	if c.c == nil {
		return nil, statusCode, fmt.Errorf("http client and options not set")
	}

	var request = map[string]interface{}{
		"method":  method,
		"params":  params,
		"id":      uint64(rand.Int63()),
		"jsonrpc": "2.0",
	}

	msg, err := encJson.Marshal(request)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", c.opts.URL(), bytes.NewBuffer(msg))
	if err != nil {
		return nil, 0, err
	}

	if c.opts.User != "" && c.opts.Password != "" {
		req.SetBasicAuth(c.opts.User, c.opts.Password)
	}

	c.c.Timeout = Timeout
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// When status is not 200 or 201, return body as error
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("%s", string(body))
	}

	// At this point, we have a successful response.
	// Decode the a map and return.
	var m map[string]interface{}
	err = json.DecodeClientResponse(resp.Body, &m)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return m, resp.StatusCode, nil
}

// makeClientStatusErr creates a StatusError representing a client error
func makeClientStatusErr(msg string, args ...interface{}) *util.StatusError {
	return util.NewStatusError(0, ErrCodeClient, "", fmt.Sprintf(msg, args...))
}

// makeStatusErrorFromCallErr converts error from a RPC call to a StatusError.
// Expects callStatusCode of 0 to indicate a client error which is expected to be non-json.
// Expects non-zero callStatusCode to be in json format and conforms to JSONRPC 2.0 error standard,
// it will panic if otherwise.
// Returns nil if err is nil
func makeStatusErrorFromCallErr(callStatusCode int, err error) *util.StatusError {
	if err == nil {
		return nil
	}

	if callStatusCode == 0 {
		return makeClientStatusErr(err.Error())
	}

	var errResp rpc.Response
	if err := encJson.Unmarshal([]byte(err.Error()), &errResp); err != nil {
		panic(errors.Wrap(err, "unable to decode call response"))
	}

	return util.NewStatusError(
		callStatusCode,
		errResp.Err.Code,
		errResp.Err.Data.(string),
		errResp.Err.Message)
}
