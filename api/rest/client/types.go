package client

import (
	"github.com/imroc/req"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// Client describes methods for accessing REST API endpoints
type RestClient interface {
	TxSendPayload(data map[string]interface{}) (*types.TxSendPayloadResponse, error)
	AccountGetNonce(address string, blockHeight ...uint64) (*types.AccountGetNonceResponse, error)
	AccountGet(address string, blockHeight ...uint64) (*state.Account, error)
	GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error)
	PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error)
	PushKeyGetNonceOfOwner(pushKeyID string, blockHeight ...uint64) (*types.AccountGetNonceResponse, error)
	PushKeyFind(pushKeyID string, blockHeight ...uint64) (*state.PushKey, error)
}
