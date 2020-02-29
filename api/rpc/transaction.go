package rpc

import (
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"
)

// TransactionAPI provides RPC methods for various local account management functionality.
type TransactionAPI struct {
	mods *modules.Modules
}

// NewTransactionAPI creates an instance of TransactionAPI
func NewTransactionAPI(mods *modules.Modules) *TransactionAPI {
	return &TransactionAPI{mods}
}

// sendPayload sends a signed transaction object to the mempool
// Body <map>: Transaction object
// Response <map>
// - hash <string>: The transaction hash
func (t *TransactionAPI) sendPayload(params interface{}) (resp *rpc.Response) {
	txMap, ok := params.(map[string]interface{})
	if !ok {
		msg := util.WrongFieldValueMsg("params", "map", params)
		msg = util.FieldError("params", msg).Error()
		return rpc.Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	}
	return rpc.Success(t.mods.Tx.SendPayload(txMap))
}

// APIs returns all API handlers
func (l *TransactionAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"sendPayload": {
			Namespace:   types.NamespaceTx,
			Description: "Sends a signed transaction object to the mempool",
			Func:        l.sendPayload,
		},
	}
}
