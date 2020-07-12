package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
	"gitlab.com/makeos/mosdef/util/crypto"
)

// NamespaceModule provides namespace management functionalities
type NamespaceModule struct {
	types.ConsoleSuggestions
	logic   core.Logic
	service services.Service
	repoMgr core.RemoteServer
}

// NewNamespaceModule creates an instance of NamespaceModule
func NewNamespaceModule(service services.Service, remoteSrv core.RemoteServer, logic core.Logic) *NamespaceModule {
	return &NamespaceModule{service: service, logic: logic, repoMgr: remoteSrv}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *NamespaceModule) ConsoleOnlyMode() bool {
	return false
}

// methods are functions exposed in the special namespace of this module.
func (m *NamespaceModule) methods() []*types.ModuleFunc {
	return []*types.ModuleFunc{
		{
			Name:        "register",
			Value:       m.Register,
			Description: "Register a namespace",
		},
		{
			Name:        "lookup",
			Value:       m.Lookup,
			Description: "Lookup a namespace",
		},
		{
			Name:        "getTarget",
			Value:       m.GetTarget,
			Description: "Lookup the target of a full namespace path",
		},
		{
			Name:        "updateDomain",
			Value:       m.UpdateDomain,
			Description: "Update one or more domains for a namespace",
		},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *NamespaceModule) globals() []*types.ModuleFunc {
	return []*types.ModuleFunc{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *NamespaceModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceNS, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceNS, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// lookup finds a namespace
//
// ARGS:
// name: The name of the namespace
// height: Optional max block height to limit the search to.
//
// RETURNS: resp <map|nil>
// resp.name <string>: The name of the namespace
// resp.expired <bool>: Indicates whether the namespace is expired
// resp.grace <bool>: Indicates whether the namespace is currently within the grace period
func (m *NamespaceModule) Lookup(name string, height ...uint64) util.Map {

	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = height[0]
	}

	ns := m.logic.NamespaceKeeper().Get(crypto.HashNamespace(name), targetHeight)
	if ns.IsNil() {
		return nil
	}

	nsMap := util.ToMap(ns)
	nsMap["name"] = name
	nsMap["expired"] = false
	nsMap["grace"] = false

	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	if ns.ExpiresAt.UInt64() <= uint64(bi.Height) {
		nsMap["expired"] = true
		if ns.GraceEndAt.UInt64() > uint64(bi.Height) {
			nsMap["grace"] = true
		}
	}

	return nsMap
}

// getTarget looks up the target of a full namespace path

// ARGS:
// path: The path to look up.
// [height]: The block height to query
//
// RETURNS: <string>: A domain's target
func (m *NamespaceModule) GetTarget(path string, height ...uint64) string {

	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = height[0]
	}

	target, err := m.logic.NamespaceKeeper().GetTarget(path, targetHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return target
}

// register a new namespace
//
// ARGS:
// params <map>
// params.name <string>:				The name of the namespace
// params.value <string>:				The cost of the namespace.
// [params.toAccount] <string>:			Set the account that will take ownership
// [params.toRepo] <string>:			Set the repo that will take ownership
// [params.domains] <map[string]string>:The initial domain->target mapping
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: The transaction hash
func (m *NamespaceModule) Register(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxNamespaceRegister()
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	// Hash the name
	tx.Name = crypto.HashNamespace(tx.Name)

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// updateDomain updates one or more domains of a namespace
//
// ARGS:
// params <map>
// params.name <string>:				The name of the namespace
// [params.domains] <map[string]string>:The initial domain->target mapping
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: The transaction hash
func (m *NamespaceModule) UpdateDomain(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxNamespaceDomainUpdate()
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	// Hash the name
	tx.Name = crypto.HashNamespace(tx.Name)

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
