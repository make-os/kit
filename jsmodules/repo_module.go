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

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	vm      *otto.Otto
	keepers types.Keepers
	service types.Service
	repoMgr types.RepoManager
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(
	vm *otto.Otto,
	service types.Service,
	repoMgr types.RepoManager,
	keepers types.Keepers) *RepoModule {
	return &RepoModule{vm: vm, service: service, keepers: keepers, repoMgr: repoMgr}
}

// funcs are functions accessible using the `repo` namespace
func (m *RepoModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "create",
			Value:       m.create,
			Description: "Create a git repository on the network",
		},
		&types.JSModuleFunc{
			Name:        "get",
			Value:       m.get,
			Description: "Find and return a repository",
		},
		&types.JSModuleFunc{
			Name:        "prune",
			Value:       m.prune,
			Description: "Delete all dangling and unreachable loose objects from a repository",
		},
		&types.JSModuleFunc{
			Name:        "upsertOwner",
			Value:       m.upsertOwner,
			Description: "Add an owner or update existing owner information",
		},
	}
}

func (m *RepoModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *RepoModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceRepo, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceRepo, f.Name)
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

// create sends a TxTypeRepoCreate transaction to create a git repository
// params {
// 		nonce: number,
//		fee: string,
// 		value: string,
//		name: string
//		timestamp: number
// }
// options: key
func (m *RepoModule) create(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxRepoCreate()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.Name = repoName.(string)
	}

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

// create sends a TxTypeRepoCreate transaction to create a git repository
// params {
// 		nonce: number,
//		fee: string,
//		name: string
// 		address: string
//		timestamp: number
// }
// options: key
func (m *RepoModule) upsertOwner(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareRepoProposalAddOwner()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.RepoName = repoName.(string)
	}

	if ownerAddr, ok := params["address"]; ok {
		defer castPanic("address")
		tx.Address = ownerAddr.(string)
	}

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

// prune removes dangling or unreachable objects from a repository.
// If force is true, the repository is immediately pruned.
func (m *RepoModule) prune(name string, force bool) {
	if force {
		if err := m.repoMgr.GetPruner().Prune(name, true); err != nil {
			panic(err)
		}
		return
	}
	m.repoMgr.GetPruner().Schedule(name)
}

// get finds and returns a repository
// name: The name of the repository
// height: Optional max block height to limit the search to.
func (m *RepoModule) get(name string, height ...uint64) interface{} {
	var targetHeight uint64
	if len(height) > 0 {
		targetHeight = uint64(height[0])
	}
	repo := m.keepers.RepoKeeper().GetRepo(name, targetHeight)
	if repo.IsNil() {
		return otto.NullValue()
	}
	return EncodeForJS(repo)
}
