package jsmodules

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// GPGModule provides gpg key management functionality
type GPGModule struct {
	cfg     *config.EngineConfig
	vm      *otto.Otto
	service types.Service
	logic   types.Logic
}

// NewGPGModule creates an instance of GPGModule
func NewGPGModule(
	cfg *config.EngineConfig,
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

func (m *GPGModule) namespacedFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "add",
			Value:       m.addPK,
			Description: "Add a GPG public key",
		},
		&types.JSModuleFunc{
			Name:        "find",
			Value:       m.find,
			Description: "Find a GPG public key by its key id",
		},
		&types.JSModuleFunc{
			Name:        "ownedBy",
			Value:       m.ownedBy,
			Description: "Get all GPG public keys belonging to an address",
		},
	}
}

func (m *GPGModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
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
// params: The parameters required to add the public key
// options: Additional call options.
// options[0]: Private key for signing the transaction
func (m *GPGModule) addPK(params map[string]interface{}, options ...interface{}) interface{} {

	var err error

	// Public key is required
	pubKey, ok := params["pubKey"]
	if !ok {
		panic(fmt.Errorf("GPG public key is required"))
	} else if _, ok = pubKey.(string); !ok {
		panic(fmt.Errorf("'pubKey' value must be a string"))
	}

	tx, key := processTxArgs(params, options...)
	tx.Type = types.TxTypeAddGPGPubKey
	tx.GPGPubKey = &types.AddGPGPubKey{
		PublicKey: pubKey.(string),
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
			panic(errors.Wrap(err, "failed to find sender's nonce"))
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

	entity, _ := crypto.PGPEntityFromPubKey(pubKey.(string))
	pkID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
		"pkID": pkID,
	})
}

// find fetches a gpg public key object by pkID
func (m *GPGModule) find(pkID string) interface{} {
	o := m.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if o.IsEmpty() {
		panic(fmt.Errorf("gpg public key not found"))
	}
	return o
}

// ownedBy returns the gpg public key ownedBy associated with the given address
func (m *GPGModule) ownedBy(address string) []string {
	return m.logic.GPGPubKeyKeeper().GetByPubKeyIDs(address)
}
