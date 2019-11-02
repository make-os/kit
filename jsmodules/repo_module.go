package jsmodules

import (
	"fmt"
	"time"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	vm      *otto.Otto
	keepers types.Keepers
	service types.Service
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(vm *otto.Otto, service types.Service, keepers types.Keepers) RepoModule {
	return RepoModule{vm: vm, service: service, keepers: keepers}
}

// funcs are functions accessible using the `repo` namespace
func (m RepoModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "create",
			Value:       m.create,
			Description: "Create a git repository",
		},
	}
}

func (m RepoModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m RepoModule) Configure() []prompt.Suggest {
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
// params: The parameters required to create the repository
// options: Additional call options.
// options[0]: Private key for signing the transaction
func (m RepoModule) create(params map[string]interface{}, options ...interface{}) interface{} {

	var err error

	// Name is required
	name, ok := params["name"]
	if !ok {
		panic(fmt.Errorf("name is required"))
	} else if _, ok = name.(string); !ok {
		panic(fmt.Errorf("'name' value must be a string"))
	}

	tx, key := processTxArgs(params, options...)
	tx.Type = types.TxTypeRepoCreate
	tx.RepoCreate = &types.RepoCreate{
		Name: name.(string),
	}

	// Set tx public key
	pk, _ := crypto.PrivKeyFromBase58(key)
	tx.SetSenderPubKey(util.String(crypto.NewKeyFromPrivKey(pk).PubKey().Base58()))

	// Set timestamp if not already set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set nonce if nonce is not provided
	if tx.Nonce == 0 {
		nonce, err := m.service.GetNonce(tx.GetFrom())
		if err != nil {
			panic(errors.Wrap(err, "failed to get sender's nonce"))
		}
		tx.Nonce = nonce + 1
	}

	// Sign the tx
	tx.Sig, err = tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
