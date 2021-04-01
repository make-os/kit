package client

import (
	"time"

	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/spf13/cast"
)

// PushKeyAPI provides access to the pushkey-related RPC methods
type PushKeyAPI struct {
	c *RPCClient
}

// GetOwner gets the account that owns the given push key
func (pk *PushKeyAPI) GetOwner(addr string, blockHeight ...uint64) (*api.ResultAccount, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	out, statusCode, err := pk.c.call("pk_getOwner", util.Map{"id": addr, "height": height})
	if err != nil {
		return nil, makeReqErrFromCallErr(statusCode, err)
	}

	r := &api.ResultAccount{Account: state.BareAccount()}
	if err = r.Account.FromMap(out); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// Register registers a public key as a push key
func (pk *PushKeyAPI) Register(body *api.BodyRegisterPushKey) (*api.ResultRegisterPushKey, error) {

	if body.SigningKey == nil {
		return nil, errors.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
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
	resp, statusCode, err := pk.c.call("pk_register", tx.ToMap())
	if err != nil {
		return nil, makeReqErrFromCallErr(statusCode, err)
	}

	var r api.ResultRegisterPushKey
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, errors.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
