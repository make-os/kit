package modules

import (
	"fmt"
	"net"
	"strconv"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc/client"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/constants"
	"github.com/stretchr/objx"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/kit/util"
	"github.com/robertkrimen/otto"
)

// RPCModule provides RPCClient functionalities
type RPCModule struct {
	types.ModuleCommon
	cfg                *config.AppConfig
	modFuncs           []*types.VMMember
	ClientContextMaker func(client types2.Client) *ClientContext
}

// NewRPCModule creates an instance of RPCModule
func NewRPCModule(cfg *config.AppConfig) *RPCModule {
	return &RPCModule{cfg: cfg, ClientContextMaker: newClientContext}
}

// methods are functions exposed in the special namespace of this module.
func (m *RPCModule) methods() []*types.VMMember {
	m.modFuncs = []*types.VMMember{
		{
			Name:        "connect",
			Value:       m.connect,
			Description: "connect to a RPC server",
		},
		{
			Name:        "local",
			Value:       m.ConnectLocal,
			Description: "connect to the local RPC server",
		},
	}

	return m.modFuncs
}

// globals are functions exposed in the VM's global namespace
func (m *RPCModule) globals() []*types.VMMember {
	return []*types.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *RPCModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Set the namespace object
	rpcNs := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceRPC, rpcNs)

	// add methods functions
	for _, f := range m.methods() {
		rpcNs[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceRPC, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// ConnectLocal returns an RPC client connected to the local RPC server
func (m *RPCModule) ConnectLocal() util.Map {
	host, port, err := net.SplitHostPort(m.cfg.Remote.Address)
	if err != nil {
		panic(err)
	}
	portInt, _ := strconv.Atoi(port)
	return m.connect(host, portInt, false, m.cfg.RPC.User, m.cfg.RPC.Password)
}

type ClientContext struct {
	Client  types2.Client
	Objects map[string]interface{}
}

// call is like callE but panics on error
func (r *ClientContext) call(methodName string, params ...interface{}) util.Map {
	out, err := r.callE(methodName, params...)
	if err != nil {
		panic(err)
	}
	return out
}

// callE calls the given RPC method and returns the error
func (r *ClientContext) callE(methodName string, params ...interface{}) (util.Map, error) {
	var p interface{}
	if len(params) > 0 {
		p = params[0]
	}
	out, _, err := r.Client.Call(methodName, p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// newClientContext creates an instance of ClientContext
func newClientContext(client types2.Client) *ClientContext {
	return &ClientContext{Client: client, Objects: map[string]interface{}{}}
}

// connect connects to an RPC server
//
//  - host: The server's IP address
//  - port: The server's port number
//  - https: Forces/Disable secure connection with server
//  - user: The server's username
//  - pass: The server's password
//
// RETURNS <map>: A mapping of rpc methods under their respective namespaces.
func (m *RPCModule) connect(host string, port int, https bool, user, pass string) util.Map {

	// Create a client
	c := client.NewClient(&types2.Options{
		Host:     host,
		Port:     port,
		HTTPS:    https,
		User:     user,
		Password: pass,
	})

	// Create a RPC context
	ctx := m.ClientContextMaker(c)

	// Add call function for raw calls
	ctx.Objects["call"] = ctx.call

	// Attempt to query the methods from the RPC server.
	// Panics if not successful.
	methods := ctx.call("rpc_methods")

	// Build RPC namespaces and add methods into them
	for _, method := range methods["methods"].([]interface{}) {
		o := objx.New(method)
		namespace := o.Get("namespace").String()
		name := o.Get("name").String()
		nsObj, ok := ctx.Objects[namespace]
		if !ok {
			nsObj = make(map[string]interface{})
			ctx.Objects[namespace] = nsObj
		}
		nsObj.(map[string]interface{})[name] = func(params ...interface{}) interface{} {
			return ctx.call(fmt.Sprintf("%s_%s", namespace, name), params...)
		}
	}

	return ctx.Objects
}
