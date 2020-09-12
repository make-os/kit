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

// UserAPI provides access to user-related remote APIs.
type UserAPI struct {
	c *RemoteClient
}

// GetNonce returns the nonce of an account
func (c *UserAPI) GetNonce(address string, blockHeight ...uint64) (*types.GetAccountNonceResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := V1Path(constants.NamespaceUser, types.MethodNameNonce)
	resp, err := c.c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var result types.GetAccountNonceResponse
	return &result, resp.ToJSON(&result)
}

// Get returns the account corresponding to the given address
func (c *UserAPI) Get(address string, blockHeight ...uint64) (*types.GetAccountResponse, error) {
	height := uint64(0)
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	path := V1Path(constants.NamespaceUser, types.MethodNameAccount)
	resp, err := c.c.get(path, M{"address": address, "height": height})
	if err != nil {
		return nil, err
	}

	var acct = &types.GetAccountResponse{Account: state.BareAccount()}
	if err = resp.ToJSON(acct.Account); err != nil {
		return nil, err
	}

	return acct, nil
}

// Send creates transaction to send coins to another user or a repository.
func (c *UserAPI) Send(body *types.SendCoinBody) (*types.HashResponse, error) {

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

	resp, err := c.c.post(V1Path(constants.NamespaceUser, types.MethodNameSendCoin), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}
