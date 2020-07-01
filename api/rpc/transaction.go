package rpc

import (
	types2 "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

// TransactionAPI provides RPC methods for various local account management functionality.
type TransactionAPI struct {
	mods *types2.Modules
}

// NewTransactionAPI creates an instance of TransactionAPI
func NewTransactionAPI(mods *types2.Modules) *TransactionAPI {
	return &TransactionAPI{mods}
}

// sendPayload sends a signed transaction object to the mempool
// Body <map>: Transaction object
// Response <map>
// - hash <string>: The transaction hash
func (t *TransactionAPI) sendPayload(params interface{}) (resp *rpc.Response) {
	txMap, ok := params.(map[string]interface{})
	if !ok {
		msg := util.FieldError("params", util.WrongFieldValueMsg("map", params)).Error()
		return rpc.Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	}
	return rpc.Success(t.mods.Tx.SendPayload(txMap))
}

// APIs returns all API handlers
func (t *TransactionAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"sendPayload": {
			Namespace:   constants.NamespaceTx,
			Description: "Sends a signed transaction object to the mempool",
			Func:        t.sendPayload,
		},
	}
}
