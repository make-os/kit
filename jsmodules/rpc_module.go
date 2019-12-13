package jsmodules

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/makeos/mosdef/rpc"
	"github.com/makeos/mosdef/rpc/client"

	"github.com/makeos/mosdef/config"

	"github.com/makeos/mosdef/util"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
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

func (m *RPCModule) namespacedFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "isRunning",
			Value:       m.isRunning,
			Description: "Checks whether the local RPC server is running",
		},
		&types.JSModuleFunc{
			Name:        "connect",
			Value:       m.connect,
			Description: "Connect to an RPC server",
		},
		&types.JSModuleFunc{
			Name:        "local",
			Value:       m.local(),
			Description: "Call methods of the local RPC server",
		},
	}
}

func (m *RPCModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
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

	// If the local rpc server is initialized, get the supported
	// methods and use them to create suggestions
	if m.rpcServer != nil {
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
func (m *RPCModule) connect(host string, port int, https bool, user, pass string) rpcMethods {

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
		out, err := c.Call(methodName, p)
		if err != nil {
			panic(err)
		}
		return out
	}

	rpcNs := make(map[string]interface{})
	rpcNs["call"] = callFunc

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
