package client

import (
	"bytes"
	encJson "encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gorilla/rpc/v2/json"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
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

type callerFunc func(method string, params interface{}) (res util.Map, statusCode int, err error)

// RPCClient provides the ability to interact with a JSON-RPC 2.0 service
type RPCClient struct {
	c    *http.Client
	opts *types.Options
	call callerFunc
}

// NewClient creates an instance of Client
func NewClient(opts *types.Options) *RPCClient {

	if opts == nil {
		opts = &types.Options{}
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
func (c *RPCClient) GetOptions() *types.Options {
	return c.opts
}

// Chain exposes methods for accessing chain information
func (c *RPCClient) Node() types.Node {
	return &ChainAPI{c: c}
}

// PushKey exposes methods for managing push keys
func (c *RPCClient) PushKey() types.PushKey {
	return &PushKeyAPI{c: c}
}

// Pool exposes methods for managing push keys
func (c *RPCClient) Pool() types.Pool {
	return &PoolAPI{c: c}
}

// Repo exposes methods for managing repositories
func (c *RPCClient) Repo() types.Repo {
	return &RepoAPI{c: c}
}

// RPC exposes methods for managing the RPC server
func (c *RPCClient) RPC() types.RPC {
	return &RPCAPI{c: c}
}

// Tx exposes methods for creating and accessing the transactions
func (c *RPCClient) Tx() types.Tx {
	return &TxAPI{c: c}
}

// User exposes methods for accessing user information
func (c *RPCClient) User() types.User {
	return &UserAPI{c: c}
}

// DHT exposes methods for accessing the DHT network
func (c *RPCClient) DHT() types.DHT {
	return &DHTAPI{c: c}
}

// Ticket exposes methods for purchasing and managing tickets
func (c *RPCClient) Ticket() types.Ticket {
	return &TicketAPI{c: c}
}

// Call calls a method on the RPCClient service.
//
// RETURNS:
//  - res: JSON-RPC 2.0 success response
//  - statusCode: RPCServer response code
//  - err: Client error or JSON-RPC 2.0 error response.
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
		return nil, 500, errors.ReqErr(500, ErrCodeConnect, "", err.Error())
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

// makeClientReqErr creates a ReqError representing a client error
func makeClientReqErr(msg string, args ...interface{}) *errors.ReqError {
	return errors.ReqErr(0, ErrCodeClient, "", fmt.Sprintf(msg, args...))
}

// makeReqErrFromCallErr converts a json-encoded error to ReqError.
// If err does not contain a JSON body, an ErrCodeUnexpected ReqError that includes
// the error message is returned.
func makeReqErrFromCallErr(callStatusCode int, err error) *errors.ReqError {
	if err == nil {
		return nil
	}

	if !govalidator.IsJSON(err.Error()) {
		se := errors.ReqErrorFromStr(err.Error())
		if se.IsSet() {
			return se
		}
		return errors.ReqErr(callStatusCode, ErrCodeUnexpected, "", err.Error())
	}

	var errResp rpc.Response
	encJson.Unmarshal([]byte(err.Error()), &errResp)

	data := ""
	if errResp.Err.Data != nil {
		data = errResp.Err.Data.(string)
	}

	return errors.ReqErr(callStatusCode, errResp.Err.Code, data, errResp.Err.Message)
}
