package jsmodules

import (
	"encoding/hex"
	"fmt"
	"strings"
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
	vm      *otto.Otto
	keepers types.Keepers
	service types.Service
}

// NewTxModule creates an instance of TxModule
func NewTxModule(vm *otto.Otto, service types.Service, keepers types.Keepers) *TxModule {
	return &TxModule{vm: vm, service: service, keepers: keepers}
}

// txCoinFuncs are functions accessible using the `tx.coin` namespace
func (m *TxModule) txCoinFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "send",
			Value:       m.sendCoin,
			Description: "Send coins to another account",
		},
	}
}

// funcs are functions accessible using the `tx` namespace
func (m *TxModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "get",
			Value:       m.get,
			Description: "Get a transactions by hash",
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

	// Add 'coin' namespaced functions
	coinMap := map[string]interface{}{}
	txMap[types.NamespaceCoin] = coinMap
	for _, f := range m.txCoinFuncs() {
		coinMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s.%s", types.NamespaceTx, types.NamespaceCoin, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add other funcs to `tx` namespace
	for _, f := range m.funcs() {
		txMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceTx, f.Name)
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
	var tx = types.NewBareTx(types.TxTypeTransferCoin)
	if err = mapstructure.Decode(txObj, tx); err != nil {
		panic(errors.Wrap(err, types.ErrArgDecode("types.Transaction", 0).Error()))
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
	hash, err := m.service.SendCoin(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// get fetches a tx by its hash
func (m *TxModule) get(hash string) interface{} {

	if strings.ToLower(hash[:2]) == "0x" {
		hash = hash[2:]
	}

	// decode the hash from hex to byte
	bz, err := hex.DecodeString(hash)
	if err != nil {
		panic(errors.Wrap(err, "invalid transaction hash"))
	}

	tx, err := m.keepers.TxKeeper().GetTx(bz)
	if err != nil {
		panic(err)
	}
	return tx
}
