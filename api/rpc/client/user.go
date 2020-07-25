package client

import (
	"time"

	"github.com/spf13/cast"
	"gitlab.com/makeos/lobe/api/types"
	"gitlab.com/makeos/lobe/types/state"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

// GetAccount gets an account corresponding to a given address
func (c *RPCClient) GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, statusCode, err := c.call("user_get", util.Map{"address": address, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	r := &types.GetAccountResponse{Account: state.BareAccount()}
	if err = r.Account.FromMap(resp); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// SendCoin creates a new repository
func (c *RPCClient) SendCoin(body *types.SendCoinBody) (*types.HashResponse, error) {

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

	resp, statusCode, err := c.call("user_send", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.HashResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}
