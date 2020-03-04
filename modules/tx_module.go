package modules

import (
	"encoding/hex"
	"fmt"
	"strings"

	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// TxModule provides transaction functionalities to JS environment
type TxModule struct {
	vm      *otto.Otto
	logic   core.Logic
	service services.Service
}

// NewTxModule creates an instance of TxModule
func NewTxModule(vm *otto.Otto, service services.Service, logic core.Logic) *TxModule {
	return &TxModule{vm: vm, service: service, logic: logic}
}

// txCoinFuncs are functions accessible using the `tx.coin` namespace
func (m *TxModule) txCoinFuncs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
		{
			Name:        "send",
			Value:       m.SendCoin,
			Description: "Send coins to another account",
		},
	}
}

// funcs are functions accessible using the `tx` namespace
func (m *TxModule) funcs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
		{
			Name:        "get",
			Value:       m.Get,
			Description: "Get a transactions by its hash",
		},
		{
			Name:        "sendPayload",
			Value:       m.SendPayload,
			Description: "Send a signed transaction payload to the network",
		},
	}
}

func (m *TxModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
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

// sendCoin sends the native coin from a source account to a destination account.
//
// ARGS:
// params <map>
// params.value 		<string>: 			The amount of coin to send
// params.to 			<string>: 			The address of the recipient
// params.nonce 		<number|string>: 	The senders next account nonce
// params.fee 			<number|string>: 	The transaction fee to pay
// params.timestamp 	<number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TxModule) SendCoin(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = core.NewBareTxCoinTransfer()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}

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

// get returns a tx by hash
func (m *TxModule) Get(hash string) util.Map {

	if strings.ToLower(hash[:2]) == "0x" {
		hash = hash[2:]
	}

	// decode the hash from hex to byte
	bz, err := hex.DecodeString(hash)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "hash", "invalid transaction hash"))
	}

	tx, err := m.logic.TxKeeper().GetTx(bz)
	if err != nil {
		if err == types.ErrTxNotFound {
			panic(util.NewStatusError(404, StatusCodeTxNotFound, "hash", types.ErrTxNotFound.Error()))
		}
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	return EncodeForJS(tx)
}

// sendPayload sends an already signed transaction object to the network
//
// ARGS:
// params: The transaction data
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TxModule) SendPayload(params map[string]interface{}) util.Map {
	tx, err := core.DecodeTxFromMap(params)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "params", err.Error()))
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		se := util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error())
		if bfe := util.BadFieldErrorFromStr(err.Error()); bfe.Msg != "" && bfe.Field != "" {
			se.Msg = bfe.Msg
			se.Field = bfe.Field
		}
		panic(se)
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
