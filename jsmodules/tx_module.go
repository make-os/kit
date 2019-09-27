package jsmodules

import (
	"fmt"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

// TxModule provides transaction functionalities to JS environment
type TxModule struct {
	vm          *otto.Otto
	nodeService types.Service
}

// NewTxModule creates an instance of TxModule
func NewTxModule(vm *otto.Otto, nodeService types.Service) *TxModule {
	return &TxModule{vm: vm, nodeService: nodeService}
}

// funcs are functions accessible using the tx.coin namespace
func (m *TxModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "send",
			Value:       m.sendCoin,
			Description: "Send the native coin from an account to a destination account.",
		},
	}
}

func (m *TxModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *TxModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main tx namespace
	txMap := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceTx, txMap)

	// add 'coin' namespaced functions
	coinMap := map[string]interface{}{}
	txMap[types.NamespaceCoin] = coinMap
	for _, f := range m.funcs() {
		coinMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s.%s", types.NamespaceTx, types.NamespaceCoin, f.Name)
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

// sendCoin sends the native coin from a source account
// to a destination account. It returns an object containing
// the hash of the transaction. It panics when an error occurs.
func (m *TxModule) sendCoin(txObj interface{}, options ...interface{}) interface{} {

	var err error

	// Decode parameters into a transaction object
	var tx types.Transaction
	if err = mapstructure.Decode(txObj, &tx); err != nil {
		panic(types.ErrArgDecode("object", 0))
	}

	// - Expect options[0] to be the private key (base58 encoded)
	// - options[0] must be a string
	// - options[0] must be a valid key
	var key string
	var ok bool
	if len(options) > 0 {
		key, ok = options[0].(string)
		if !ok {
			panic(types.ErrArgDecode("string", 1))
		} else if err := crypto.IsValidPrivKey(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	} else {
		panic(fmt.Errorf("key is required"))
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
		nonce, err := m.nodeService.GetNonce(tx.GetFrom())
		if err != nil {
			panic(errors.Wrap(err, "failed to get sender's nonce"))
		}
		tx.Nonce = nonce + 1
	}

	// Compute the hash
	tx.SetHash(tx.ComputeHash())

	// Sign the tx
	tx.Sig, err = tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}

	// Process the transaction
	hash, err := m.nodeService.SendCoin(&tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
