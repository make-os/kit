package modules

import (
	"fmt"
	prompt "github.com/c-bata/go-prompt"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	types3 "gitlab.com/makeos/mosdef/services/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// NamespaceModule provides namespace management functionalities
type NamespaceModule struct {
	vm      *otto.Otto
	keepers core.Keepers
	service types3.Service
	repoMgr core.RepoManager
}

// NewNSModule creates an instance of NamespaceModule
func NewNSModule(
	vm *otto.Otto,
	service types3.Service,
	repoMgr core.RepoManager,
	keepers core.Keepers) *NamespaceModule {
	return &NamespaceModule{vm: vm, service: service, keepers: keepers, repoMgr: repoMgr}
}

// funcs are functions accessible using the `ns` namespace
func (m *NamespaceModule) funcs() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		{
			Name:        "register",
			Value:       m.register,
			Description: "Register a namespace",
		},
		{
			Name:        "lookup",
			Value:       m.lookup,
			Description: "Lookup a namespace",
		},
		{
			Name:        "getTarget",
			Value:       m.getTarget,
			Description: "Lookup the target of a full namespace path",
		},
		{
			Name:        "updateDomain",
			Value:       m.updateDomain,
			Description: "Update one or more domains for a namespace",
		},
	}
}

func (m *NamespaceModule) globals() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *NamespaceModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceNS, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceNS, f.Name)
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
func (m *NamespaceModule) lookup(name string, height ...uint64) interface{} {

	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = uint64(height[0])
	}

	ns := m.keepers.NamespaceKeeper().GetNamespace(util.Hash20Hex([]byte(name)), targetHeight)
	if ns.IsNil() {
		return nil
	}
	nsMap := util.StructToJSON(ns)
	nsMap["name"] = name
	nsMap["expired"] = false
	nsMap["expiring"] = false

	curBlockInfo, err := m.keepers.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(err)
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
func (m *NamespaceModule) getTarget(path string, height ...uint64) string {

	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = uint64(height[0])
	}

	target, err := m.keepers.NamespaceKeeper().GetTarget(path, targetHeight)
	if err != nil {
		panic(err)
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
func (m *NamespaceModule) register(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxNamespaceAcquire()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	// Hash the name
	tx.Name = util.Hash20Hex([]byte(tx.Name))

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
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
func (m *NamespaceModule) updateDomain(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxNamespaceDomainUpdate()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	// Hash the name
	tx.Name = util.Hash20Hex([]byte(tx.Name))

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
