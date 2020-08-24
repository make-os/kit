package client

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

	"github.com/asaskevich/govalidator"
	"github.com/gorilla/rpc/v2/json"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/util"
)

// Timeout is the max duration for connection and read attempt
const (
	Timeout             = 15 * time.Second
	ErrCodeClient       = "client_error"
	ErrCodeDecodeFailed = "decode_error"
	ErrCodeUnexpected   = "unexpected_error"
	ErrCodeConnect      = "connect_error"
	ErrCodeBadParam     = "bad_param_error"
)

// Client represents a JSON-RPC client
type Client interface {
	SendTxPayload(data map[string]interface{}) (*types.HashResponse, error)
	GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, error)
	GetTransaction(hash string) (*types.GetTxResponse, error)
	GetPushKeyOwner(id string, blockHeight ...uint64) (*types.GetAccountResponse, error)
	RegisterPushKey(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error)
	CreateRepo(body *types.CreateRepoBody) (*types.CreateRepoResponse, error)
	VoteRepoProposal(body *types.RepoVoteBody) (*types.HashResponse, error)
	GetRepo(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error)
	AddRepoContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error)
	SendCoin(body *types.SendCoinBody) (*types.HashResponse, error)
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
		opts.Host = "0.0.0.0"
	}

	client := &RPCClient{c: new(http.Client), opts: opts}
	client.call = client.Call

	return client
}

// SetCallFunc sets the RPC call function
func (c *RPCClient) SetCallFunc(f callerFunc) {
	c.call = f
}

// GetOptions returns the client's option
func (c *RPCClient) GetOptions() *Options {
	return c.opts
}

// Call calls a method on the RPCClient service.
// Returns:
// res: JSON-RPC 2.0 success response
// statusCode: RPCServer response code
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
		return nil, 500, util.ReqErr(500, ErrCodeConnect, "", err.Error())
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

// makeClientStatusErr creates a ReqError representing a client error
func makeClientStatusErr(msg string, args ...interface{}) *util.ReqError {
	return util.ReqErr(0, ErrCodeClient, "", fmt.Sprintf(msg, args...))
}

// makeStatusErrorFromCallErr converts error containing a JSON marshalled
// status error to ReqError. If error does not contain a JSON object,
// an ErrCodeUnexpected status error including the error message is returned.
func makeStatusErrorFromCallErr(callStatusCode int, err error) *util.ReqError {
	if err == nil {
		return nil
	}

	// For non-json error, return an ErrCodeUnexpected status error
	if !govalidator.IsJSON(err.Error()) {
		se := util.ReqErrorFromStr(err.Error())
		if se.IsSet() {
			return se
		}
		return util.ReqErr(callStatusCode, ErrCodeUnexpected, "", err.Error())
	}

	var errResp rpc.Response
	encJson.Unmarshal([]byte(err.Error()), &errResp)

	data := ""
	if errResp.Err.Data != nil {
		data = errResp.Err.Data.(string)
	}

	return util.ReqErr(callStatusCode, errResp.Err.Code, data, errResp.Err.Message)
}
