package modules

import (
	"fmt"

	"github.com/makeos/mosdef/config"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/util"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// GPGModule provides gpg key management functionality
type GPGModule struct {
	cfg     *config.AppConfig
	vm      *otto.Otto
	service types.Service
	logic   types.Logic
}

// NewGPGModule creates an instance of GPGModule
func NewGPGModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	service types.Service,
	logic types.Logic) *GPGModule {
	return &GPGModule{
		cfg:     cfg,
		vm:      vm,
		service: service,
		logic:   logic,
	}
}

func (m *GPGModule) namespacedFuncs() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{
		&types.ModulesAggregatorFunc{
			Name:        "add",
			Value:       m.addPK,
			Description: "Add a GPG public key",
		},
		&types.ModulesAggregatorFunc{
			Name:        "find",
			Value:       m.find,
			Description: "Find a GPG public key by its key ID",
		},
		&types.ModulesAggregatorFunc{
			Name:        "ownedBy",
			Value:       m.ownedBy,
			Description: "Get all GPG public keys belonging to an address",
		},
	}
}

func (m *GPGModule) globals() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{}
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
// params {
// 		nonce: number,
//		fee: string,
//		pubKey: string
//		timestamp: number
// }
// options: key
func (m *GPGModule) addPK(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxAddGPGPubKey()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if pubKey, ok := params["pubKey"]; ok {
		defer castPanic("pubKey")
		tx.PublicKey = pubKey.(string)
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

// find fetches a gpg public key object by pkID
func (m *GPGModule) find(pkID string) interface{} {
	o := m.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if o.IsNil() {
		panic(fmt.Errorf("gpg public key not found"))
	}
	return o
}

// ownedBy returns the gpg public key ownedBy associated with the given address
func (m *GPGModule) ownedBy(address string) []string {
	return m.logic.GPGPubKeyKeeper().GetPubKeyIDs(address)
}
