package client

import (
	"time"

	"github.com/spf13/cast"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// GetPushKeyOwner gets the account that owns a push key
func (c *RPCClient) GetPushKeyOwner(id string, blockHeight ...uint64) (*types.GetAccountResponse, *util.ReqError) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	out, statusCode, err := c.call("pk_getOwner", util.Map{"id": id, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	r := &types.GetAccountResponse{Account: state.BareAccount()}
	if err = r.Account.FromMap(out); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// RegisterPushKey creates a new repository
func (c *RPCClient) RegisterPushKey(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
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

	// call RPC method: repo_create
	resp, statusCode, err := c.call("pk_register", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.RegisterPushKeyResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}
