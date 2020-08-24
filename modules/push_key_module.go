package modules

import (
	"fmt"

	"github.com/make-os/lobe/api/rpc/client"
	apitypes "github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	modulestypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/node/services"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/spf13/cast"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// PushKeyModule manages and provides access to push keys.
type PushKeyModule struct {
	modulestypes.ModuleCommon
	cfg     *config.AppConfig
	service services.Service
	logic   core.Logic
}

// NewAttachablePushKeyModule creates an instance of PushKeyModule suitable in attach mode
func NewAttachablePushKeyModule(client client.Client) *PushKeyModule {
	return &PushKeyModule{ModuleCommon: modulestypes.ModuleCommon{AttachedClient: client}}
}

// NewPushKeyModule creates an instance of PushKeyModule
func NewPushKeyModule(cfg *config.AppConfig, service services.Service, logic core.Logic) *PushKeyModule {
	return &PushKeyModule{cfg: cfg, service: service, logic: logic}
}

// methods are functions exposed in the special namespace of this module.
func (m *PushKeyModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
		{
			Name:        "register",
			Value:       m.Register,
			Description: "Register a push key",
		},
		{
			Name:        "unregister",
			Value:       m.Unregister,
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
			Name:        "getOwner",
			Value:       m.GetAccountOfOwner,
			Description: "Get the account of a push key owner",
		},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *PushKeyModule) globals() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *PushKeyModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Set the namespace object
	nsMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespacePushKey, nsMap)

	// add methods functions
	for _, f := range m.methods() {
		nsMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespacePushKey, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
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
// object.address <string>: 			The  push key address
func (m *PushKeyModule) Register(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxRegisterPushKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	printPayload, signingKey := finalizeTx(tx, m.logic, m.AttachedClient, options...)
	if printPayload {
		return tx.ToMap()
	}

	if m.InAttachMode() {
		resp, err := m.AttachedClient.RegisterPushKey(&apitypes.RegisterPushKeyBody{
			PublicKey:  tx.PublicKey,
			Scopes:     tx.Scopes,
			FeeCap:     cast.ToFloat64(tx.FeeCap.String()),
			Nonce:      tx.Nonce,
			Fee:        cast.ToFloat64(tx.Fee.String()),
			SigningKey: crypto.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	pk := crypto.MustPubKeyFromBytes(tx.PublicKey.Bytes())

	return map[string]interface{}{
		"hash":    hash,
		"address": pk.PushAddr().String(),
	}
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
	var tx = txns.NewBareTxUpDelPushKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}
	tx.Delete = false

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	// Process the transaction
	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// Unregister removes a push key from the network
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
func (m *PushKeyModule) Unregister(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	// Decode parameters into a transaction object
	var tx = txns.NewBareTxUpDelPushKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}
	tx.Delete = true
	tx.FeeCap = ""
	tx.AddScopes = nil
	tx.RemoveScopes = nil

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	// Process the transaction
	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// Get fetches a push key by its address
//
// ARGS:
// address: 	he push key address
// [height]: 	The target block height to query (default: latest)
//
// RETURNS state.PushKey
func (m *PushKeyModule) Get(address string, height ...uint64) util.Map {

	if address == "" {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "id", "push key id is required"))
	}

	h := uint64(0)
	if len(height) > 0 {
		h = height[0]
	}

	o := m.logic.PushKeyKeeper().Get(address, h)
	if o.IsNil() {
		panic(util.ReqErr(404, StatusCodePushKeyNotFound, "", types.ErrPushKeyUnknown.Error()))
	}

	return util.ToMap(o)
}

// GetByAddress returns a list of push key addresses owned by the given user address
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
// address: The push key address
// [height]: 	The target block height to query (default: latest)
//
// RETURNS state.Account
func (m *PushKeyModule) GetAccountOfOwner(address string, height ...uint64) util.Map {

	h := uint64(0)
	if len(height) > 0 {
		h = height[0]
	}

	if m.InAttachMode() {
		resp, err := m.AttachedClient.GetPushKeyOwner(address, h)
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	pushKey := m.Get(address, height...)
	acct := m.logic.AccountKeeper().Get(pushKey["address"].(identifier.Address), h)
	if acct.IsNil() {
		panic(util.ReqErr(404, StatusCodeAccountNotFound, "address", types.ErrAccountUnknown.Error()))
	}

	return util.ToMap(acct)
}
