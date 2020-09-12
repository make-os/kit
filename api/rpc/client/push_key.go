package client

import (
	"time"

	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
)

// PushKeyAPI provides access to the pushkey-related RPC methods
type PushKeyAPI struct {
	client *RPCClient
}

// GetOwner gets the account that owns the given push key
func (pk *PushKeyAPI) GetOwner(addr string, blockHeight ...uint64) (*types.GetAccountResponse, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	out, statusCode, err := pk.client.call("pk_getOwner", util.Map{"id": addr, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	r := &types.GetAccountResponse{Account: state.BareAccount()}
	if err = r.Account.FromMap(out); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// Register registers a public key as a push key
func (pk *PushKeyAPI) Register(body *types.RegisterPushKeyBody) (*types.RegisterPushKeyResponse, error) {

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
	resp, statusCode, err := pk.client.call("pk_register", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.RegisterPushKeyResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}
