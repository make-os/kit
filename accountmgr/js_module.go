package accountmgr

import (
	"fmt"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// JSModuleName is the name of the global variable by
// which users will access the functionalities of the module
const JSModuleName = "account"

// JSModule provides functionalities that are accessible
// through the javascript console environment
type JSModule struct {
	acctMgr *AccountManager
	vm      *otto.Otto
}

// NewJSModule creates an instance of JSModule for account management
func NewJSModule(acctmgr *AccountManager) *JSModule {
	return &JSModule{
		acctMgr: acctmgr,
	}
}

func (m *JSModule) namespacedFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "listAccounts",
			Value:       m.listAccounts,
			Description: "Fetch all accounts that exist on this node",
		},
	}
}

func (m *JSModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "accounts",
			Value:       m.listAccounts(),
			Description: "Get the list of accounts that exist on this node",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *JSModule) Configure(vm *otto.Otto) []prompt.Suggest {
	m.vm = vm
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", JSModuleName, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	vm.Set(JSModuleName, fMap)

	// Add global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
	}

	return suggestions
}

// listAccounts lists all accounts on this node
func (m *JSModule) listAccounts() []string {
	accounts, err := m.acctMgr.ListAccounts()
	if err != nil {
		panic(err)
	}

	var resp = []string{}
	for _, a := range accounts {
		resp = append(resp, a.Address)
	}

	return resp
}
