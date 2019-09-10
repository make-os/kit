package node

import (
	"fmt"

	prompt "github.com/c-bata/go-prompt"
	"github.com/k0kubun/pp"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

const jsModuleName = "tx"
const jsCoinMapName = "coin"

// JSModule provides functionalities that are accessible
// through the javascript console environment
type JSModule struct {
	vm      *otto.Otto
	service types.Service
}

// NewJSModule creates an instance of JSModule for account management.
// Pass the node service so it can perform node specific operations.
func NewJSModule(nodeService types.Service) *JSModule {
	return &JSModule{
		service: nodeService,
	}
}

// namespaceCoinFuncs are functions accessible using the tx.coin namespace
func (m *JSModule) namespaceCoinFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "send",
			Value:       m.sendCoin,
			Description: "Send the native coin from an account to a destination account.",
		},
	}
}

func (m *JSModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *JSModule) Configure(vm *otto.Otto) []prompt.Suggest {
	m.vm = vm
	suggestions := []prompt.Suggest{}
	txMap := map[string]interface{}{}

	// add 'coin' namespaced functions
	coinMap := map[string]interface{}{}
	txMap[jsCoinMapName] = coinMap
	for _, f := range m.namespaceCoinFuncs() {
		coinMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s.%s", jsModuleName, jsCoinMapName, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add the main tx namespace
	vm.Set(jsModuleName, txMap)

	// Add global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
	}

	return suggestions
}

// sendCoin sends the native coin from a source account
// to a destination account
func (m *JSModule) sendCoin() {
	// m.service.Do(SrvNameCoinSend, "abc")
	pp.Println("Args", params)
}
