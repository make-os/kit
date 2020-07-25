package client

import (
	"fmt"
	"time"

	"github.com/spf13/cast"
	"github.com/themakeos/lobe/api/remote"
	"github.com/themakeos/lobe/api/types"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
)

// GetPushKeyOwnerNonce returns the nonce of the push key owner account
func (c *ClientV1) GetPushKeyOwnerNonce(pushKeyID string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := c.get(remote.V1Path(constants.NamespacePushKey, types.MethodNameOwnerNonce), params)
	if err != nil {
		return nil, err
	}

	var result types.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetPushKey finds a push key by its ID.
// If blockHeight is specified, only the block at the given height is searched.
func (c *ClientV1) GetPushKey(pushKeyID string, blockHeight ...uint64) (*types.GetPushKeyResponse, error) {

	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	params := M{"id": pushKeyID, "height": height}
	resp, err := c.get(remote.V1Path(constants.NamespacePushKey, types.MethodNamePushKeyFind), params)
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
func (c *ClientV1) RegisterPushKey(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error) {
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

	resp, err := c.post(remote.V1Path(constants.NamespacePushKey, types.MethodNamePushKeyRegister), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.RegisterPushKeyResponse
	return &result, resp.ToJSON(&result)
}
