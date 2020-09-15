package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/make-os/lobe/util"
)

// Params represent JSON API parameters
type Params map[string]interface{}

// Scan attempts to convert the params to a struct or map type
func (p *Params) Scan(dest interface{}) error {
	return util.DecodeMap(p, &dest)
}

// CallContext contains information about an RPC request
type CallContext struct {

	// IsLocal indicates that the request originated locally
	IsLocal bool
}

type Method func(params interface{}) *Response
type MethodWithContext func(params interface{}, ctx *CallContext) *Response

// MethodInfo describes an RPC method.
type MethodInfo struct {

	// Func is the API function to be executed.
	// Must be Method or MethodWithContext
	Func interface{} `json:"-"`

	// Namespace is the namespace where the method is under
	Namespace string `json:"namespace"`

	// Name is the name of the method
	Name string `json:"name"`

	// Private indicates a requirement for a private, authenticated
	// user session before this API function is executed.
	Private bool `json:"private"`

	// Description describes the API
	Description string `json:"description"`
}

func (a *MethodInfo) FullName() string {
	return fmt.Sprintf("%s_%s", a.Namespace, a.Name)
}

// APISet defines a collection of APIs
type APISet []MethodInfo

// Get gets an API function by name
// and namespace
func (a *APISet) Get(name string) *MethodInfo {
	for _, v := range *a {
		if name == v.FullName() {
			return &v
		}
	}
	return nil
}

// Get gets an API function by name
// and namespace
func (a *APISet) Add(api MethodInfo) {
	*a = append(*a, api)
}

// API defines an interface for providing and
// accessing API functions. Packages that offer
// services accessed via RPC or any service-oriented
// interface must implement it.
type API interface {
	APIs() APISet
}

// OnRequestFunc is the type of function to use
// as a callback when newRPCHandler requests are received
type OnRequestFunc func(r *http.Request) error

// Request represent a JSON RPC request
type Request struct {
	JSONRPCVersion string      `json:"jsonrpc"`
	Method         string      `json:"method"`
	Params         interface{} `json:"params"`
	ID             interface{} `json:"id,omitempty"`
}

// IsNotification checks whether the request is a notification
// according to JSON RPC specification.
// When ID is nil, we assume it's a notification request.
func (r Request) IsNotification() bool {
	if r.ID == nil {
		return true
	}

	switch v := r.ID.(type) {
	case string:
		return v == "0"
	case float64:
		return v == 0
	default:
		panic(fmt.Errorf("id has unexpected type"))
	}
}

// Err represents JSON RPC error object
type Err struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Response represents a JSON RPC response
type Response struct {
	JSONRPCVersion string      `json:"jsonrpc"`
	Result         util.Map    `json:"result"`
	Err            *Err        `json:"error,omitempty"`
	ID             interface{} `json:"id,omitempty"` // string or float64
}

// IsError checks whether r is an error response
func (r Response) IsError() bool {
	return r.Err != nil
}

// ToJSON returns the JSON encoding of r
func (r Response) ToJSON() []byte {
	bz, _ := json.Marshal(r)
	return bz
}

// Error creates an error response
func Error(code interface{}, message string, data interface{}) *Response {
	return &Response{
		JSONRPCVersion: "2.0",
		Err:            &Err{Code: fmt.Sprintf("%v", code), Message: message, Data: data},
	}
}

// Success creates a success response
func Success(result util.Map) *Response {
	return &Response{JSONRPCVersion: "2.0", Result: result}
}

// StatusOK creates a success response with data `{status:true}`
func StatusOK() *Response {
	return &Response{JSONRPCVersion: "2.0", Result: util.Map{"status": true}}
}
