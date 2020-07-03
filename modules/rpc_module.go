package modules

import (
	"fmt"
	"net"
	"strconv"

	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/console"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/rpc/api/client"
	"gitlab.com/makeos/mosdef/types/constants"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// RPCModule provides RPCClient functionalities
type RPCModule struct {
	console.ConsoleSuggestions
	cfg       *config.AppConfig
	rpcServer *rpc.Server
	modFuncs  []*types.ModuleFunc
}

// NewRPCModule creates an instance of RPCModule
func NewRPCModule(cfg *config.AppConfig, rpcServer *rpc.Server) *RPCModule {
	return &RPCModule{cfg: cfg, rpcServer: rpcServer}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *RPCModule) ConsoleOnlyMode() bool {
	return true
}

// methods are functions exposed in the special namespace of this module.
func (m *RPCModule) methods() []*types.ModuleFunc {
	m.modFuncs = []*types.ModuleFunc{
		{
			Name:        "isRunning",
			Value:       m.IsRunning,
			Description: "Checks whether the local RPCClient server is running",
		},
		{
			Name:        "connect",
			Value:       m.Connect,
			Description: "Connect to a RPC server",
		},
		{
			Name:        "local",
			Value:       m.ConnectLocal,
			Description: "Connect to the local RPC server",
		},
	}

	return m.modFuncs
}

// globals are functions exposed in the VM's global namespace
func (m *RPCModule) globals() []*types.ModuleFunc {
	return []*types.ModuleFunc{}
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

// isRunning checks whether the server is running
func (m *RPCModule) IsRunning() bool {
	return m.rpcServer != nil && m.rpcServer.IsRunning()
}

// ConnectLocal returns an RPC client connected to the local RPC server
func (m *RPCModule) ConnectLocal() util.Map {
	host, port, err := net.SplitHostPort(m.cfg.RPC.Address)
	if err != nil {
		panic(err)
	}
	portInt, _ := strconv.Atoi(port)
	return m.Connect(host, portInt, false, m.cfg.RPC.User, m.cfg.RPC.Password)
}

type RPCContext struct {
	client  client.Client
	objects map[string]interface{}
}

// call is like callE but panics on error
func (r *RPCContext) call(methodName string, params ...interface{}) interface{} {
	out, err := r.callE(methodName, params...)
	if err != nil {
		panic(err)
	}
	return out
}

// callE calls the given RPC method and returns the error
func (r *RPCContext) callE(methodName string, params ...interface{}) (interface{}, error) {
	var p interface{}
	if len(params) > 0 {
		p = params[0]
	}
	out, _, err := r.client.Call(methodName, p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Connect connects to an RPC server
//
// ARGS
// host: The server's IP address
// port: The server's port number
// https: Forces/Disable secure connection with server
// user: The server's username
// pass: The server's password
//
// RETURNS <map>: A mapping of rpc methods under their respective namespaces.
func (m *RPCModule) Connect(host string, port int, https bool, user, pass string) util.Map {

	// Create a client
	c := client.NewClient(&client.Options{
		Host:     host,
		Port:     port,
		HTTPS:    https,
		User:     user,
		Password: pass,
	})

	// Create a RPC context
	ctx := &RPCContext{client: c, objects: map[string]interface{}{}}

	// Add call function for raw calls
	ctx.objects["call"] = ctx.call

	// Attempt to query the methods from the RPC server.
	// Panics if not successful.
	methods := ctx.call("rpc_methods")

	// Build RPC namespaces and add methods into their respective namespaces
	for _, method := range methods.(util.Map)["methods"].([]interface{}) {
		o := objx.New(method)
		namespace := o.Get("namespace").String()
		name := o.Get("name").String()
		nsObj, ok := ctx.objects[namespace]
		if !ok {
			nsObj = make(map[string]interface{})
			ctx.objects[namespace] = nsObj
		}
		nsObj.(map[string]interface{})[name] = func(params ...interface{}) interface{} {
			return ctx.call(fmt.Sprintf("%s_%s", namespace, name), params...)
		}
	}

	return ctx.objects
}
