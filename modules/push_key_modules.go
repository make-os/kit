package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// PushKeyModule manages and provides access to push keys.
type PushKeyModule struct {
	cfg     *config.AppConfig
	vm      *otto.Otto
	service services.Service
	logic   core.Logic
}

// NewPushKeyModule creates an instance of PushKeyModule
func NewPushKeyModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	service services.Service,
	logic core.Logic) *PushKeyModule {
	return &PushKeyModule{
		cfg:     cfg,
		vm:      vm,
		service: service,
		logic:   logic,
	}
}

func (m *PushKeyModule) namespacedFuncs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
		{
			Name:        "register",
			Value:       m.Register,
			Description: "Register a push key",
		},
		{
			Name:        "unregister",
			Value:       m.UnRegister,
			Description: "Remove a push key from the network",
		},
		{
			Name:        "update",
			Value:       m.Update,
			Description: "Update a previously registered push key",
		},
		{
			Name:        "get",
			Value:       m.Get,
			Description: "Get a push key by its key unique ID",
		},
		{
			Name:        "getByAddress",
			Value:       m.GetByAddress,
			Description: "Get registered push keys belonging to an address",
		},
		{
			Name:        "getAccountOfOwner",
			Value:       m.GetAccountOfOwner,
			Description: "Get the account of a push key owner",
		},
	}
}

func (m *PushKeyModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *PushKeyModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	var suggestions []prompt.Suggest

	// Set the namespace object
	util.VMSet(m.vm, constants.NamespacePushKey, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespacePushKey, f.Name)
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

// Register registers a push key with the network.
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
// params.pubKey <string>:				The public key
// params.scopes <string|[]string>:		A list of repo or namespace where the key can be used.
// params.feeCap <number|string>:		The max. amount of fee the key can spend
//
// options <[]interface{}>
// options[0] key 			<string>: 	The signer's private key
// options[1] payloadOnly 	<bool>: 	When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: 				The transaction hash
// object.pushKeyID <string>: 			The unique network ID of the push key
func (m *PushKeyModule) Register(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxRegisterPushKey()
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

	pk := crypto.MustPubKeyFromBytes(tx.PublicKey.Bytes())

	return EncodeForJS(map[string]interface{}{
		"hash":    hash,
		"address": pk.PushAddr().String(),
	})
}

// Update updates a push key
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
// params.id <string>:					The target push key ID
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
func (m *PushKeyModule) Update(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxUpDelPushKey()
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

// UnRegister removes a push key from the network
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
// params.id <string>:					The target push key ID
//
// options <[]interface{}>
// options[0] key 			<string>: 	The signer's private key
// options[1] payloadOnly 	<bool>: 	When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: 				The transaction hash
func (m *PushKeyModule) UnRegister(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = core.NewBareTxUpDelPushKey()
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

// Get fetches a push key by id
//
// ARGS:
// id: 				The push key ID
// [blockHeight]: 	The target block height to query (default: latest)
//
// RETURNS state.PushKey
func (m *PushKeyModule) Get(id string, blockHeight ...uint64) util.Map {

	if id == "" {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "id", "push key id is required"))
	}

	targetHeight := uint64(0)
	if len(blockHeight) > 0 {
		targetHeight = blockHeight[0]
	}

	o := m.logic.PushKeyKeeper().Get(id, targetHeight)
	if o.IsNil() {
		panic(util.NewStatusError(404, StatusCodePushKeyNotFound, "", types.ErrPushKeyUnknown.Error()))
	}

	return EncodeForJS(o)
}

// ownedBy fetches push keys owned by the given address
//
// ARGS:
// address: An address of an account
//
// RETURNS: List of push key ids
func (m *PushKeyModule) GetByAddress(address string) []string {
	return m.logic.PushKeyKeeper().GetByAddress(address)
}

// GetAccountOfOwner returns the account of the key owner
//
// ARGS:
// pushKeyID: The push key id
// [blockHeight]: 	The target block height to query (default: latest)
//
// RETURNS state.Account
func (m *PushKeyModule) GetAccountOfOwner(pushKeyID string, blockHeight ...uint64) util.Map {
	pushKey := m.Get(pushKeyID, blockHeight...)

	targetHeight := uint64(0)
	if len(blockHeight) > 0 {
		targetHeight = blockHeight[0]
	}

	acct := m.logic.AccountKeeper().Get(
		pushKey["address"].(util.Address),
		targetHeight)
	if acct.IsNil() {
		panic(util.NewStatusError(404, StatusCodeAccountNotFound, "pushKeyID", types.ErrAccountUnknown.Error()))
	}

	return EncodeForJS(acct)
}
