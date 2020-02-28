package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
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
	service services.Service
	logic   core.Logic
}

// NewGPGModule creates an instance of GPGModule
func NewGPGModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	service services.Service,
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
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.commission <number|string>:	The network commission value
// params.timestamp <number>: 			The unix timestamp
// params.pubKey <string>:				The GPG public key
//
// options <[]interface{}>
// options[0] key 			<string>: 	The signer's private key
// options[1] payloadOnly 	<bool>: 	When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: The transaction hash
func (m *GPGModule) addPK(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxAddGPGPubKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	// Process the transaction
	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// Find fetches a GPG public key object by id
//
// ARGS:
// id: 				The public key ID to search for
// [blockHeight]: 	The target block height to query (default: latest)
//
// RETURNS state.GPGPubKey
func (m *GPGModule) Find(id string, blockHeight ...uint64) *state.GPGPubKey {

	targetHeight := uint64(0)
	if len(blockHeight) > 0 {
		targetHeight = blockHeight[0]
	}

	o := m.logic.GPGPubKeyKeeper().GetGPGPubKey(id, targetHeight)
	if o.IsNil() {
		panic(util.NewStatusError(404, StatusCodeGPGPubKeyNotFound, "", types.ErrGPGPubKeyUnknown.Error()))
	}

	return o
}

// ownedBy returns the gpg public key ownedBy associated with the given address
//
// ARGS:
// address: An address of an account
//
// RETURNS: List of GPG public key ids
func (m *GPGModule) ownedBy(address string) []string {
	return m.logic.GPGPubKeyKeeper().GetPubKeyIDs(address)
}

// GetAccountOfOwner returns the account of the key owner
//
// ARGS:
// gpgID: The GPG key id
// [blockHeight]: 	The target block height to query (default: latest)
//
// RETURNS state.Account
func (m *GPGModule) GetAccountOfOwner(gpgID string, blockHeight ...uint64) *state.Account {
	gpgKey := m.Find(gpgID, blockHeight...)

	targetHeight := uint64(0)
	if len(blockHeight) > 0 {
		targetHeight = blockHeight[0]
	}

	acct := m.logic.AccountKeeper().GetAccount(gpgKey.Address, targetHeight)
	if acct.IsNil() {
		panic(util.NewStatusError(404, StatusCodeAccountNotFound, "gpgID", types.ErrAccountUnknown.Error()))
	}

	return acct
}
