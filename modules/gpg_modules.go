package modules

import (
	"fmt"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	servtypes "gitlab.com/makeos/mosdef/services/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"

	prompt "github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// GPGModule provides gpg key management functionality
type GPGModule struct {
	cfg     *config.AppConfig
	vm      *otto.Otto
	service servtypes.Service
	logic   core.Logic
}

// NewGPGModule creates an instance of GPGModule
func NewGPGModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	service servtypes.Service,
	logic core.Logic) *GPGModule {
	return &GPGModule{
		cfg:     cfg,
		vm:      vm,
		service: service,
		logic:   logic,
	}
}

func (m *GPGModule) namespacedFuncs() []*modulestypes.ModulesAggregatorFunc {
	return []*modulestypes.ModulesAggregatorFunc{
		{
			Name:        "add",
			Value:       m.addPK,
			Description: "Add a GPG public key",
		},
		{
			Name:        "find",
			Value:       m.Find,
			Description: "Find a GPG public key by its key ID",
		},
		{
			Name:        "ownedBy",
			Value:       m.ownedBy,
			Description: "Get all GPG public keys belonging to an address",
		},
		{
			Name:        "getAccountOfOwner",
			Value:       m.GetAccountOfOwner,
			Description: "Get the account of the key owner",
		},
	}
}

func (m *GPGModule) globals() []*modulestypes.ModulesAggregatorFunc {
	return []*modulestypes.ModulesAggregatorFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *GPGModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// Set the namespace object
	util.VMSet(m.vm, types.NamespaceGPG, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceGPG, f.Name)
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

// addPk adds a GPG public key to an account.
// params {
// 		nonce: number,
//		fee: string,
//		pubKey: string
//		timestamp: number
// }
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *GPGModule) addPK(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxAddGPGPubKey()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// Find fetches a gpg public key object by pkID
func (m *GPGModule) Find(pkID string) *state.GPGPubKey {
	o := m.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if o.IsNil() {
		panic(fmt.Errorf("gpg public key not found"))
	}
	return o
}

// ownedBy returns the gpg public key ownedBy associated with the given address
func (m *GPGModule) ownedBy(address string) []string {
	return m.logic.GPGPubKeyKeeper().GetPubKeyIDs(address)
}

// GetAccountOfOwner returns the account of the key owner
func (m *GPGModule) GetAccountOfOwner(pkID string) *state.Account {
	gpgKey := m.Find(pkID)
	acct := m.logic.AccountKeeper().GetAccount(gpgKey.Address)
	if acct.IsNil() {
		panic(types.ErrAccountUnknown)
	}
	return acct
}
