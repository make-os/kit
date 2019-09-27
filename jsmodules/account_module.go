package jsmodules

import (
	"fmt"

	"github.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	"github.com/makeos/mosdef/accountmgr"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// AccountModule provides account management functionalities
// that are accessed through the javascript console environment
type AccountModule struct {
	acctMgr *accountmgr.AccountManager
	vm      *otto.Otto
	service types.Service
}

// NewAccountModule creates an instance of AccountModule
func NewAccountModule(
	vm *otto.Otto,
	acctmgr *accountmgr.AccountManager,
	service types.Service) *AccountModule {
	return &AccountModule{
		acctMgr: acctmgr,
		vm:      vm,
		service: service,
	}
}

func (m *AccountModule) namespacedFuncs() []*types.JSModuleFunc {
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
		&types.JSModuleFunc{
			Name:        "getNonce",
			Value:       m.getNonce,
			Description: "Get the nonce of an account",
		},
	}
}

func (m *AccountModule) globals() []*types.JSModuleFunc {
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
func (m *AccountModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// Set the namespace object
	util.VMSet(m.vm, types.NamespaceAccount, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceAccount, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// listAccounts lists all accounts on this node
func (m *AccountModule) listAccounts() []string {
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
func (m *AccountModule) getKey(address string, passphrase ...string) string {

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

// getNonce returns the current nonce of an account
func (m *AccountModule) getNonce(address string) string {
	nonce, err := m.service.GetNonce(util.String(address))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d", nonce)
}
