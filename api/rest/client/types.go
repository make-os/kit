package client

import (
	"github.com/imroc/req"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// Client describes methods for accessing REST API endpoints
type Client interface {
	SendTxPayload(data map[string]interface{}) (*types.SendTxPayloadResponse, error)
	GetAccountNonce(address string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error)
	GetAccount(address string, blockHeight ...uint64) (*state.Account, error)
	GetCall(endpoint string, params map[string]interface{}) (*req.Resp, error)
	PostCall(endpoint string, body map[string]interface{}) (*req.Resp, error)
	GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error)
	GetPushKey(pushKeyID string, blockHeight ...uint64) (*state.PushKey, error)
}
