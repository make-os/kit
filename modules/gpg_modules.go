package modules

import (
	"crypto/rsa"
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
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

func (m *GPGModule) namespacedFuncs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
		{
			Name:        "register",
			Value:       m.Register,
			Description: "Register a GPG public key",
		},
		{
			Name:        "unregister",
			Value:       m.UnRegister,
			Description: "Remove a GPG public key from the network",
		},
		{
			Name:        "update",
			Value:       m.Update,
			Description: "Update a previously registered GPG key",
		},
		{
			Name:        "get",
			Value:       m.Get,
			Description: "Get a GPG public key by its key ID",
		},
		{
			Name:        "getByAddress",
			Value:       m.GetByAddress,
			Description: "Get registered GPG public keys belonging to an address",
		},
		{
			Name:        "getAccountOfOwner",
			Value:       m.GetAccountOfOwner,
			Description: "Get the account of the key owner",
		},
	}
}

func (m *GPGModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
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

	// Register global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// Register adds a GPG public key to an account.
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
// params.pubKey <string>:				The GPG public key
// params.scopes <string|[]string>:		A list of repo or namespace where the key can be used.
// params.feeCap <number|string>:		The max. amount of fee the key can spend
//
// options <[]interface{}>
// options[0] key 			<string>: 	The signer's private key
// options[1] payloadOnly 	<bool>: 	When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: 				The transaction hash
// object.gpgID <string>: 				The unique network ID of the GPG key
func (m *GPGModule) Register(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxRegisterGPGPubKey()
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

	entity, _ := crypto.PGPEntityFromPubKey(tx.PublicKey)
	gpgID := util.CreateGPGIDFromRSA(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	return EncodeForJS(map[string]interface{}{
		"hash":  hash,
		"gpgID": gpgID,
	})
}

// Update updates a GPG public key
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
// params.id <string>:					The target GPG public key ID
// params.addScopes <string|[]string>:	Register a repo names or namespace where the key can be used.
// params.removeScopes <int|[]int>:		Select indices of existing scopes to be deleted.
// params.feeCap <number|string>:		The max. amount of fee the key can spend
//
// options <[]interface{}>
// options[0] key 			<string>: 	The signer's private key
// options[1] payloadOnly 	<bool>: 	When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: 				The transaction hash
func (m *GPGModule) Update(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxUpDelGPGPubKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}
	tx.Delete = false

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

// UnRegister removes a GPG public key from the network
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
// params.id <string>:					The target GPG public key ID
//
// options <[]interface{}>
// options[0] key 			<string>: 	The signer's private key
// options[1] payloadOnly 	<bool>: 	When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: 				The transaction hash
func (m *GPGModule) UnRegister(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxUpDelGPGPubKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}
	tx.Delete = true
	tx.FeeCap = ""
	tx.AddScopes = nil
	tx.RemoveScopes = nil

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

// Get fetches a GPG public key object by id
//
// ARGS:
// id: 				The public key ID to search for
// [blockHeight]: 	The target block height to query (default: latest)
//
// RETURNS state.GPGPubKey
func (m *GPGModule) Get(id string, blockHeight ...uint64) util.Map {

	if id == "" {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "id", "gpg id is required"))
	}

	targetHeight := uint64(0)
	if len(blockHeight) > 0 {
		targetHeight = blockHeight[0]
	}

	o := m.logic.GPGPubKeyKeeper().Get(id, targetHeight)
	if o.IsNil() {
		panic(util.NewStatusError(404, StatusCodeGPGPubKeyNotFound, "", types.ErrGPGPubKeyUnknown.Error()))
	}

	return EncodeForJS(o)
}

// ownedBy returns the gpg public key ownedBy associated with the given address
//
// ARGS:
// address: An address of an account
//
// RETURNS: List of GPG public key ids
func (m *GPGModule) GetByAddress(address string) []string {
	return m.logic.GPGPubKeyKeeper().GetByAddress(address)
}

// GetAccountOfOwner returns the account of the key owner
//
// ARGS:
// gpgID: The GPG key id
// [blockHeight]: 	The target block height to query (default: latest)
//
// RETURNS state.Account
func (m *GPGModule) GetAccountOfOwner(gpgID string, blockHeight ...uint64) util.Map {
	gpgKey := m.Get(gpgID, blockHeight...)

	targetHeight := uint64(0)
	if len(blockHeight) > 0 {
		targetHeight = blockHeight[0]
	}

	acct := m.logic.AccountKeeper().Get(
		gpgKey["address"].(util.Address),
		targetHeight)
	if acct.IsNil() {
		panic(util.NewStatusError(404, StatusCodeAccountNotFound, "gpgID", types.ErrAccountUnknown.Error()))
	}

	return EncodeForJS(acct)
}
