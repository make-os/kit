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

// GetAccountNonce returns the nonce of the given address
func (c *ClientV1) GetAccountNonce(address string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := remote.V1Path(constants.NamespaceUser, types.MethodNameNonce)
	resp, err := c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var result types.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// GetAccount returns the account corresponding to the given address
func (c *ClientV1) GetAccount(address string, blockHeight ...uint64) (*types.GetAccountResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := remote.V1Path(constants.NamespaceUser, types.MethodNameAccount)
	resp, err := c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var acct = &types.GetAccountResponse{Account: state.BareAccount()}
	if err = resp.ToJSON(acct.Account); err != nil {
		return nil, err
	}

	return acct, nil
}

// SendCoin creates transaction to send coins to another user or a repository.
func (c *ClientV1) SendCoin(body *types.SendCoinBody) (*types.HashResponse, error) {

	if body.SigningKey == nil {
		return nil, fmt.Errorf("signing key is required")
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
		return nil, err
	}

	resp, err := c.post(remote.V1Path(constants.NamespaceUser, types.MethodNameSendCoin), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}
