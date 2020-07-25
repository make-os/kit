package rpc

import (
	types2 "gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/rpc"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/util"
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
func (t *TransactionAPI) sendPayload(params interface{}) (resp *rpc.Response) {
	txMap, ok := params.(map[string]interface{})
	if !ok {
		msg := util.FieldError("params", util.WrongFieldValueMsg("map", params)).Error()
		return rpc.Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	}
	return rpc.Success(t.mods.Tx.SendPayload(txMap))
}

// getTransaction gets a transaction by its hash
func (a *TransactionAPI) getTransaction(params interface{}) (resp *rpc.Response) {
	hash, ok := params.(string)
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a string", "")
	}
	return rpc.Success(a.mods.Tx.Get(hash))
}

// APIs returns all API handlers
func (t *TransactionAPI) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{
			Name:        "sendPayload",
			Namespace:   constants.NamespaceTx,
			Description: "Sends a signed transaction payload to the mempool",
			Func:        t.sendPayload,
		},
		{
			Name:        "get",
			Namespace:   constants.NamespaceTx,
			Description: "Get a transaction by its hash",
			Func:        t.getTransaction,
		},
	}
}
