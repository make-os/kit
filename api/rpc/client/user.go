package client

import (
	"time"

	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
)

// UserAPI provides access to user-related RPC methods
type UserAPI struct {
	client *RPCClient
}

// Get gets an account corresponding to a given address
func (u *UserAPI) Get(address string, blockHeight ...uint64) (*types.ResultAccount, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, statusCode, err := u.client.call("user_get", util.Map{"address": address, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	r := &types.ResultAccount{Account: state.BareAccount()}
	if err = r.Account.FromMap(resp); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// Send sends coins from a user account to another account or repository
func (u *UserAPI) Send(body *types.BodySendCoin) (*types.ResultHash, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareTxCoinTransfer()
	tx.Nonce = body.Nonce
	tx.Value = util.String(cast.ToString(body.Value))
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.To = body.To
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privKey", err.Error())
	}

	resp, statusCode, err := u.client.call("user_send", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.ResultHash
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}
