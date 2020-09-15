package api

import (
	types2 "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/types/constants"
	"github.com/spf13/cast"
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
	return rpc.Success(t.mods.Tx.SendPayload(cast.ToStringMap(params)))
}

// getTransaction gets a transaction by its hash
func (a *TransactionAPI) getTransaction(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Tx.Get(cast.ToString(params)))
}

// APIs returns all API handlers
func (t *TransactionAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:        "send",
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
