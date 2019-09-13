package jsmodule

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/makeos/mosdef/accountmgr"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// JSModuleName is the name of the global variable by
// which users will access the functionalities of the module
const JSModuleName = "account"

// Module provides functionalities that are accessible
// through the javascript console environment
type Module struct {
	acctMgr *accountmgr.AccountManager
	vm      *otto.Otto
}

// NewModule creates an instance of Module for account management
func NewModule(acctmgr *accountmgr.AccountManager) *Module {
	return &Module{
		acctMgr: acctmgr,
	}
}

func (m *Module) namespacedFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "listAccounts",
			Value:       m.listAccounts,
			Description: "Fetch all accounts that exist on this node",
		},
		&types.JSModuleFunc{
			Name:        "getKey",
			Value:       m.getKey,
			Description: "Get the private key of an account (supports interactive mode)",
		},
	}
}

func (m *Module) globals() []*types.JSModuleFunc {
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
func (m *Module) Configure(vm *otto.Otto) []prompt.Suggest {
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
func (m *Module) listAccounts() []string {
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

// getKey returns the private key of a given key.
// The passphrase argument is used to unlock the address.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
func (m *Module) getKey(address string, passphrase ...string) string {

	var pass string

	if address == "undefined" {
		panic(fmt.Errorf("address is required"))
	}

	// Find the address
	acct, err := m.acctMgr.GetByAddress(address)
	if err != nil {
		panic(err)
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass, err = m.acctMgr.AskForPasswordOnce()
		if err != nil {
			panic(err)
		}
	} else {
		pass = passphrase[0]
	}

	// Decrypt the account using the passphrase
	if err := acct.Decrypt(pass); err != nil {
		panic(errors.Wrap(err, "failed to unlock account with the provided passphrase"))
	}

	return acct.GetKey().PrivKey().Base58()
}
