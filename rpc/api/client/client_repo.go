package client

import (
	"time"

	"gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// CreateRepo creates a new repository
func (c *RPCClient) CreateRepo(body *api.CreateRepoBody) (*api.CreateRepoResponse, error) {

	if body.SigningKey == nil {
		return nil, util.StatusErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	// Create a TxRepoCreate object and fill it with args
	tx := txns.NewBareTxRepoCreate()
	tx.Name = body.Name
	tx.Nonce = body.Nonce
	tx.Value = util.String(body.Value)
	tx.Fee = util.String(body.Fee)
	tx.Timestamp = time.Now().Unix()
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()
	if body.Config != nil {
		tx.Config = body.Config.ToMap()
	}

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.StatusErr(400, ErrCodeClient, "privKey", err.Error())
	}

	// call RPC method: repo_create
	resp, statusCode, err := c.call("repo_create", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r api.CreateRepoResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}

// GetRepo finds and returns a repository
func (c *RPCClient) GetRepo(name string, opts ...*api.GetRepoOpts) (*api.GetRepoResponse, *util.StatusError) {

	if len(opts) == 0 {
		opts = []*api.GetRepoOpts{{}}
	}

	params := util.Map{"name": name, "height": opts[0].Height, "noProposals": opts[0].NoProposals}
	resp, statusCode, err := c.call("repo_get", params)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = api.GetRepoResponse{Repository: state.BareRepository()}
	if err := util.DecodeMap(resp, r.Repository); err != nil {
		return nil, util.StatusErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
