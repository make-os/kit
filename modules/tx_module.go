package modules

import (
	"fmt"

	"github.com/make-os/lobe/api/rpc/client"
	modulestypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/node/services"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/lobe/util"
	"github.com/robertkrimen/otto"
)

const (
	TxStatusInMempool  = "in_mempool"
	TxStatusInPushpool = "in_pushpool"
	TxStatusInBlock    = "in_block"
)

// TxModule provides transaction functionalities to JS environment
type TxModule struct {
	modulestypes.ModuleCommon
	logic   core.Logic
	service services.Service
}

// NewTxModule creates an instance of TxModule
func NewTxModule(service services.Service, logic core.Logic) *TxModule {
	return &TxModule{service: service, logic: logic}
}

// NewAttachableTxModule creates an instance of TxModule suitable in attach mode
func NewAttachableTxModule(client client.Client) *TxModule {
	return &TxModule{ModuleCommon: modulestypes.ModuleCommon{AttachedClient: client}}
}

// methods are functions exposed in the special namespace of this module.
func (m *TxModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
		{Name: "get", Value: m.Get, Description: "Get a transactions by its hash"},
		{Name: "send", Value: m.SendPayload, Description: "Send a signed transaction payload to the network"},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *TxModule) globals() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *TxModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main tx namespace
	txMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceTx, txMap)

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

// Get returns a tx by hash
//
// ARGS:
// hash: The transaction hash
//
// RETURNS object 	<map>
// object.status 	<string>: 		The status of the transaction (in_block, in_mempool or in_pushpool).
// object.data		<object>: 		The transaction object.
func (m *TxModule) Get(hash string) util.Map {

	if m.InAttachMode() {
		tx, err := m.AttachedClient.GetTransaction(hash)
		if err != nil {
			panic(err)
		}
		return util.ToMap(tx)
	}

	bz, err := util.FromHex(hash)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "hash", "invalid transaction hash"))
	}

	// Check tx in transaction index (finalized check)
	tx, err := m.logic.TxKeeper().GetTx(bz)
	if err != nil && err != types.ErrTxNotFound {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	} else if tx != nil {
		return map[string]interface{}{"status": TxStatusInBlock, "data": util.ToMap(tx)}
	}

	// Check tx in the mempool
	if tx := m.logic.GetMempoolReactor().GetTx(hash); tx != nil {
		return map[string]interface{}{"status": TxStatusInMempool, "data": util.ToMap(tx)}
	}

	// Check tx in push pool
	if note := m.logic.GetRemoteServer().GetPushPool().Get(hash); note != nil {
		return map[string]interface{}{"status": TxStatusInPushpool, "data": note.ToMap()}
	}

	panic(util.ReqErr(404, StatusCodeTxNotFound, "hash", types.ErrTxNotFound.Error()))
}

// sendPayload sends an already signed transaction object to the network
//
// ARGS:
// params: The transaction data
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TxModule) SendPayload(params map[string]interface{}) util.Map {

	if m.InAttachMode() {
		tx, err := m.AttachedClient.SendTxPayload(params)
		if err != nil {
			panic(err)
		}
		return util.ToMap(tx)
	}

	tx, err := txns.DecodeTxFromMap(params)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		se := util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error())
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
