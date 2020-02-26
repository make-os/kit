package modules

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"gitlab.com/makeos/mosdef/types"

	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/rpc"

	"gitlab.com/makeos/mosdef/config"

	prompt "github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/util"
)

// RPCModule provides RPC functionalities
type RPCModule struct {
	cfg       *config.AppConfig
	vm        *otto.Otto
	rpcServer *rpc.Server
}

// NewRPCModule creates an instance of RPCModule
func NewRPCModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	rpcServer *rpc.Server) *RPCModule {
	return &RPCModule{
		cfg:       cfg,
		vm:        vm,
		rpcServer: rpcServer,
	}
}

func (m *RPCModule) namespacedFuncs() []*modtypes.ModulesAggregatorFunc {
	modFuncs := []*modtypes.ModulesAggregatorFunc{
		{
			Name:        "isRunning",
			Value:       m.isRunning,
			Description: "Checks whether the local RPC server is running",
		},
		{
			Name:        "connect",
			Value:       m.connect,
			Description: "Connect to an RPC server",
		},
	}

	if !m.cfg.ConsoleOnly() {
		modFuncs = append(modFuncs, &modtypes.ModulesAggregatorFunc{
			Name:        "local",
			Value:       m.local(),
			Description: "Call methods of the local RPC server",
		})
	}

	return modFuncs
}

func (m *RPCModule) globals() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *RPCModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// Set the namespace object
	util.VMSet(m.vm, types.NamespaceRPC, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceRPC, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{
			Text:        f.Name,
			Description: f.Description,
		})
	}

	// If the local rpc server is initialized and  we are not in console-only mode,
	// get the supported methods and use them to create rpc suggestions under 'local' namespace
	if m.rpcServer != nil && !m.cfg.ConsoleOnly() {
		for _, method := range m.rpcServer.GetMethods() {
			funcFullName := fmt.Sprintf("%s.local.%s", types.NamespaceRPC,
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
func (m *RPCModule) isRunning() bool {
	return m.rpcServer != nil && m.rpcServer.IsRunning()
}

type rpcMethods map[string]interface{}

func (m *RPCModule) local() rpcMethods {
	host, port, err := net.SplitHostPort(m.cfg.RPC.Address)
	if err != nil {
		panic(err)
	}
	portInt, _ := strconv.Atoi(port)
	return m.connect(host, portInt, false, m.cfg.RPC.User, m.cfg.RPC.Password)
}

// connect to an RPC server
//
// ARGS
// host: The server's IP address
// port: The server's port number
// https: Forces/Disable secure connection with server
// user: The server's username
// pass: The server's password
//
// RETURNS <map>: A mapping of rpc methods and call functions
func (m *RPCModule) connect(host string, port int, https bool, user, pass string) rpcMethods {

	c := client.NewClient(&rpc.Options{
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
		out, err := c.Call(methodName, p)
		if err != nil {
			panic(err)
		}
		return out
	}

	rpcNs := make(map[string]interface{})
	rpcNs["call"] = callFunc

	// Fill the rpc namespace with convenience methods that allow calls such
	// as namespace.method(param).
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
	} else {
		methods, err := c.Call("rpc_methods", nil)
		if err == nil {
			for _, method := range methods.([]interface{}) {
				fullName := method.(map[string]interface{})["name"].(string)
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
	}

	return rpcNs
}
