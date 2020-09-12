package client

import (
	"fmt"
	"time"

	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
)

// PushKeyAPI provides access to the pushkey-related remote APIs.
type PushKeyAPI struct {
	c *RemoteClient
}

// GetPushKeyOwnerNonce returns the nonce of the push key owner account
func (a *PushKeyAPI) GetOwnerNonce(pushKeyID string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := a.c.get(V1Path(constants.NamespacePushKey, types.MethodNameOwnerNonce), params)
	if err != nil {
		return nil, err
	}

	var result types.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// Get finds a push key by its ID.
// If blockHeight is specified, only the block at the given height is searched.
func (a *PushKeyAPI) Get(pushKeyID string, blockHeight ...uint64) (*types.GetPushKeyResponse, error) {

	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := a.c.get(V1Path(constants.NamespacePushKey, types.MethodNamePushKeyFind), params)
	if err != nil {
		return nil, err
	}

	var pk = &types.GetPushKeyResponse{PushKey: state.BarePushKey()}
	if err = resp.ToJSON(pk.PushKey); err != nil {
		return nil, err
	}

	return pk, nil
}

// Register creates a transaction to register a push key
func (a *PushKeyAPI) Register(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error) {
	if body.SigningKey == nil {
		return nil, fmt.Errorf("signing key is required")
	}

	tx := txns.NewBareTxRegisterPushKey()
	tx.PublicKey = body.PublicKey
	tx.Nonce = body.Nonce
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Scopes = body.Scopes
	tx.Timestamp = time.Now().Unix()
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()
	if body.FeeCap > 0 {
		tx.FeeCap = util.String(cast.ToString(body.FeeCap))
	}

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, err
	}

	resp, err := a.c.post(V1Path(constants.NamespacePushKey, types.MethodNamePushKeyRegister), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.RegisterPushKeyResponse
	return &result, resp.ToJSON(&result)
}
