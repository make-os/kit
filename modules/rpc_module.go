package modules

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/constants"

	"gitlab.com/makeos/mosdef/config"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// RPCModule provides RPCClient functionalities
type RPCModule struct {
	cfg       *config.AppConfig
	ctx       *types.ModulesContext
	rpcServer *rpc.Server
}

// NewRPCModule creates an instance of RPCModule
func NewRPCModule(cfg *config.AppConfig, rpcServer *rpc.Server) *RPCModule {
	return &RPCModule{cfg: cfg, rpcServer: rpcServer, ctx: types.DefaultModuleContext}
}

// SetContext sets the function used to retrieve call context
func (m *RPCModule) SetContext(cg *types.ModulesContext) {
	m.ctx = cg
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *RPCModule) ConsoleOnlyMode() bool {
	return true
}

// methods are functions exposed in the special namespace of this module.
func (m *RPCModule) methods() []*types.ModuleFunc {
	modFuncs := []*types.ModuleFunc{
		{
			Name:        "isRunning",
			Value:       m.IsRunning,
			Description: "Checks whether the local RPCClient server is running",
		},
		{
			Name:        "connect",
			Value:       m.Connect,
			Description: "Connect to an RPCClient server",
		},
	}

	if !m.cfg.ConsoleOnly() {
		modFuncs = append(modFuncs, &types.ModuleFunc{
			Name:        "local",
			Value:       m.Local(),
			Description: "Call methods of the local RPCClient server",
		})
	}

	return modFuncs
}

// globals are functions exposed in the VM's global namespace
func (m *RPCModule) globals() []*types.ModuleFunc {
	return []*types.ModuleFunc{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *RPCModule) ConfigureVM(vm *otto.Otto) []prompt.Suggest {
	fMap := map[string]interface{}{}
	var suggestions []prompt.Suggest

	// Set the namespace object
	util.VMSet(vm, constants.NamespaceRPC, fMap)

	// add methods functions
	for _, f := range m.methods() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceRPC, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{
			Text:        f.Name,
			Description: f.Description,
		})
	}

	// If the local rpc server is initialized and  we are not in console-only mode,
	// get the supported methods and use them to create rpc suggestions under 'local' namespace
	if m.rpcServer != nil && !m.cfg.ConsoleOnly() {
		for _, method := range m.rpcServer.GetMethods() {
			funcFullName := fmt.Sprintf("%s.local.%s", constants.NamespaceRPC,
				strings.ReplaceAll(method.Name, "_", "."))
			suggestions = append(suggestions, prompt.Suggest{
				Text:        funcFullName,
				Description: method.Description,
			})
		}
	}

	return suggestions
}

// isRunning checks whether the server is running
func (m *RPCModule) IsRunning() bool {
	return m.rpcServer != nil && m.rpcServer.IsRunning()
}

func (m *RPCModule) Local() util.Map {
	host, port, err := net.SplitHostPort(m.cfg.RPC.Address)
	if err != nil {
		panic(err)
	}
	portInt, _ := strconv.Atoi(port)
	return m.Connect(host, portInt, false, m.cfg.RPC.User, m.cfg.RPC.Password)
}

// connect to an RPCClient server
//
// ARGS
// host: The server's IP address
// port: The server's port number
// https: Forces/Disable secure connection with server
// user: The server's username
// pass: The server's password
//
// RETURNS <map>: A mapping of rpc methods and call functions
func (m *RPCModule) Connect(host string, port int, https bool, user, pass string) util.Map {

	c := client.NewClient(&client.Options{
		Host:     host,
		Port:     port,
		HTTPS:    https,
		User:     user,
		Password: pass,
	})

	var callFunc = func(methodName string, params ...interface{}) interface{} {
		var p interface{}
		if len(params) > 0 {
			p = params[0]
		}
		out, _, err := c.Call(methodName, p)
		if err != nil {
			panic(err)
		}
		return out
	}

	rpcNs := make(map[string]interface{})
	rpcNs["call"] = callFunc

	// Fill the rpc namespace with convenience methods that allows calls such
	// as namespace.method(param).
	// ---
	// When the local RPC server is running, directly ask it for a collection
	// of methods it supports, and use them to create the convenience methods.
	if m.rpcServer != nil && m.rpcServer.IsRunning() {
		for _, method := range m.rpcServer.GetMethods() {
			methodName := method.Name
			ns := method.Namespace
			curNs, ok := rpcNs[ns]
			if !ok {
				curNs = make(map[string]interface{})
				rpcNs[ns] = curNs
			}
			curNs.(map[string]interface{})[strings.Split(methodName, "_")[1]] = func(
				params ...interface{}) interface{} {
				return callFunc(methodName, params...)
			}
		}
		return rpcNs
	}

	// At this point, the rpc server has not been initialized or is running.
	// So we have to ask the remote RPC server for the RPC methods via the client
	// and use the response to populate the rpc namespace with the convenience methods
	methods, _, err := c.Call("rpc_methods", nil)
	if err == nil {
		var m map[string]rpc.MethodInfo
		_ = util.DecodeMap(methods, &m)
		for _, method := range m {
			fullName := method.Name
			parts := strings.Split(fullName, "_")
			ns := parts[0]
			curNs, ok := rpcNs[ns]
			if !ok {
				curNs = make(map[string]interface{})
				rpcNs[ns] = curNs
			}
			curNs.(map[string]interface{})[parts[1]] = func(
				params ...interface{}) interface{} {
				return callFunc(fullName, params...)
			}
		}
	}

	return rpcNs
}
