package modules

import (
	"fmt"

	prompt "github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"
)

// NamespaceModule provides namespace management functionalities
type NamespaceModule struct {
	vm      *otto.Otto
	logic   core.Logic
	service services.Service
	repoMgr core.RepoManager
}

// NewNSModule creates an instance of NamespaceModule
func NewNSModule(
	vm *otto.Otto,
	service services.Service,
	repoMgr core.RepoManager,
	logic core.Logic) *NamespaceModule {
	return &NamespaceModule{vm: vm, service: service, logic: logic, repoMgr: repoMgr}
}

// funcs are functions accessible using the `ns` namespace
func (m *NamespaceModule) funcs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
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
			Description: "Lookup the target of a full namespace URI",
		},
		{
			Name:        "updateDomain",
			Value:       m.UpdateDomain,
			Description: "Update one or more domains for a namespace",
		},
	}
}

func (m *NamespaceModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *NamespaceModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, constants.NamespaceNS, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceNS, f.Name)
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

// lookup finds a namespace
//
// ARGS:
// name: The name of the namespace
// height: Optional max block height to limit the search to.
//
// RETURNS: resp <map|nil>
// resp.name <string>: The name of the namespace
// resp.expired <bool>: Indicates whether the namespace is expired
// resp.expiring <bool>: Indicates whether the namespace is currently expiring
func (m *NamespaceModule) Lookup(name string, height ...uint64) interface{} {

	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = uint64(height[0])
	}

	ns := m.logic.NamespaceKeeper().Get(util.HashNamespace(name), targetHeight)
	if ns.IsNil() {
		return nil
	}
	nsMap := util.StructToMap(ns)
	nsMap["name"] = name
	nsMap["expired"] = false
	nsMap["expiring"] = false

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	if ns.GraceEndAt <= uint64(curBlockInfo.Height) {
		nsMap["expired"] = true
	}

	if ns.ExpiresAt <= uint64(curBlockInfo.Height) {
		nsMap["expiring"] = true
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
		targetHeight = uint64(height[0])
	}

	target, err := m.logic.NamespaceKeeper().GetTarget(path, targetHeight)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
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
func (m *NamespaceModule) Register(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxNamespaceAcquire()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}

	// Hash the name
	tx.Name = util.HashNamespace(tx.Name)

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
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
func (m *NamespaceModule) UpdateDomain(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxNamespaceDomainUpdate()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}

	// Hash the name
	tx.Name = util.HashNamespace(tx.Name)

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
