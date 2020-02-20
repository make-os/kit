package rpc

import (
	"net"
	"strconv"
)

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
