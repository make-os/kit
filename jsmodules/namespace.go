package jsmodules

import (
	"fmt"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

// NamespaceModule provides namespace management functionalities
type NamespaceModule struct {
	vm      *otto.Otto
	keepers types.Keepers
	service types.Service
	repoMgr types.RepoManager
}

// NewNSModule creates an instance of NamespaceModule
func NewNSModule(
	vm *otto.Otto,
	service types.Service,
	repoMgr types.RepoManager,
	keepers types.Keepers) *NamespaceModule {
	return &NamespaceModule{vm: vm, service: service, keepers: keepers, repoMgr: repoMgr}
}

// funcs are functions accessible using the `ns` namespace
func (m *NamespaceModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "register",
			Value:       m.register,
			Description: "Register a namespace",
		},
		&types.JSModuleFunc{
			Name:        "lookup",
			Value:       m.lookup,
			Description: "Lookup a namespace",
		},
		&types.JSModuleFunc{
			Name:        "getTarget",
			Value:       m.getTarget,
			Description: "Lookup the target of a full namespace path",
		},
		&types.JSModuleFunc{
			Name:        "updateDomain",
			Value:       m.updateDomain,
			Description: "Update one or more domains for a namespace",
		},
	}
}

func (m *NamespaceModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
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
// name: The name of the namespace
// height: Optional max block height to limit the search to.
func (m *NamespaceModule) lookup(name string, height ...uint64) interface{} {

	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = uint64(height[0])
	}

	ns := m.keepers.NamespaceKeeper().GetNamespace(util.Hash20Hex([]byte(name)), targetHeight)
	if ns.IsNil() {
		return otto.NullValue()
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
// path: The path to look up.
// height: Optional max block height to limit the search to.
func (m *NamespaceModule) getTarget(path string, height ...uint64) interface{} {

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

// register sends a TxTypeNSAcquire transaction to buy a namespace
// params {
// 		nonce: number,
//		fee: string,
// 		value: string,
//		name: string
//		toAccount: string
//		toRepo: string
//		timestamp: number
//		domains: {[key:string]: string}
// }
// options: key
func (m *NamespaceModule) register(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxNamespaceAcquire()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	if namespace, ok := params["name"]; ok {
		defer castPanic("name")
		tx.Name = namespace.(string)
	}

	if trToAccount, ok := params["toAccount"]; ok {
		defer castPanic("toAccount")
		tx.TransferToAccount = trToAccount.(string)
	}

	if trToRepo, ok := params["toRepo"]; ok {
		defer castPanic("toRepo")
		tx.TransferToRepo = trToRepo.(string)
	}

	if domains, ok := params["domains"]; ok {
		defer castPanic("domains")
		domains := domains.(map[string]interface{})
		for k, v := range domains {
			tx.Domains[k] = v.(string)
		}
	}

	// Hash the name
	tx.Name = util.Hash20Hex([]byte(tx.Name))

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// updateDomain updates one or more domains of a namespace
// params {
// 		nonce: number,
//		fee: string,
//		name: string
//		timestamp: number
//		domains: {[key:string]: string}
// }
// options: key
func (m *NamespaceModule) updateDomain(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxNamespaceDomainUpdate()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if namespace, ok := params["name"]; ok {
		defer castPanic("name")
		tx.Name = namespace.(string)
	}

	if domains, ok := params["domains"]; ok {
		defer castPanic("domains")
		domains := domains.(map[string]interface{})
		for k, v := range domains {
			tx.Domains[k] = v.(string)
		}
	}

	// Hash the name
	tx.Name = util.Hash20Hex([]byte(tx.Name))

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
