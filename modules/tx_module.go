package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/console"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/txns"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// TxModule provides transaction functionalities to JS environment
type TxModule struct {
	console.ConsoleSuggestions
	logic   core.Logic
	service services.Service
}

// NewTxModule creates an instance of TxModule
func NewTxModule(service services.Service, logic core.Logic) *TxModule {
	return &TxModule{service: service, logic: logic}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *TxModule) ConsoleOnlyMode() bool {
	return false
}

// coinMethods are functions accessible using the `tx.coin` namespace
func (m *TxModule) coinMethods() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{
		{
			Name:        "send",
			Value:       m.SendCoin,
			Description: "Send coins to another account",
		},
	}
}

// methods are functions exposed in the special namespace of this module.
func (m *TxModule) methods() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{
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

// globals are functions exposed in the VM's global namespace
func (m *TxModule) globals() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *TxModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main tx namespace
	txMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceTx, txMap)

	// Register 'coin' methods functions
	coinMap := map[string]interface{}{}
	txMap[constants.NamespaceCoin] = coinMap
	for _, f := range m.coinMethods() {
		coinMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s.%s", constants.NamespaceTx, constants.NamespaceCoin, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register other methods to `tx` namespace
	for _, f := range m.methods() {
		txMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceTx, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
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

	var tx = txns.NewBareTxCoinTransfer()
	if err = tx.FromMap(params); err != nil {
		panic(util.StatusErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.StatusErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// get returns a tx by hash
func (m *TxModule) Get(hash string) util.Map {

	bz, err := util.FromHex(hash)
	if err != nil {
		panic(util.StatusErr(400, StatusCodeInvalidParam, "hash", "invalid transaction hash"))
	}

	tx, err := m.logic.TxKeeper().GetTx(bz)
	if err != nil {
		if err == types.ErrTxNotFound {
			panic(util.StatusErr(404, StatusCodeTxNotFound, "hash", types.ErrTxNotFound.Error()))
		}
		panic(util.StatusErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.StructToMap(tx)
}

// sendPayload sends an already signed transaction object to the network
//
// ARGS:
// params: The transaction data
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TxModule) SendPayload(params map[string]interface{}) util.Map {
	tx, err := txns.DecodeTxFromMap(params)
	if err != nil {
		panic(util.StatusErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		se := util.StatusErr(400, StatusCodeMempoolAddFail, "", err.Error())
		if bfe := util.BadFieldErrorFromStr(err.Error()); bfe.Msg != "" && bfe.Field != "" {
			se.Msg = bfe.Msg
			se.Field = bfe.Field
		}
		panic(se)
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
