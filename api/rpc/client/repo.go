package client

import (
	"time"

	"github.com/spf13/cast"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// CreateRepo creates a new repository
func (c *RPCClient) CreateRepo(body *types.CreateRepoBody) (*types.CreateRepoResponse, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	// Create a TxRepoCreate object and fill it with args
	tx := txns.NewBareTxRepoCreate()
	tx.Name = body.Name
	tx.Nonce = body.Nonce
	tx.Value = util.String(cast.ToString(body.Value))
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()
	if body.Config != nil {
		tx.Config = body.Config
	}

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privKey", err.Error())
	}

	resp, statusCode, err := c.call("repo_create", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.CreateRepoResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}

// GetRepo finds and returns a repository
func (c *RPCClient) GetRepo(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error) {

	if len(opts) == 0 {
		opts = []*types.GetRepoOpts{{}}
	}

	params := util.Map{"name": name, "height": opts[0].Height, "noProposals": opts[0].NoProposals}
	resp, statusCode, err := c.call("repo_get", params)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = types.GetRepoResponse{Repository: state.BareRepository()}
	if err := util.DecodeMap(resp, r.Repository); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// AddRepoContributors creates transaction to create a add repo contributors
func (c *RPCClient) AddRepoContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareRepoProposalRegisterPushKey()
	tx.PushKeys = body.PushKeys
	tx.ID = body.ProposalID
	tx.RepoName = body.RepoName
	tx.Value = util.String(cast.ToString(body.Value))
	tx.Namespace = body.Namespace
	tx.NamespaceOnly = body.NamespaceOnly
	tx.FeeCap = util.String(cast.ToString(body.FeeCap))
	tx.FeeMode = state.FeeMode(body.FeeMode)
	tx.Nonce = body.Nonce
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()
	if body.Policies != nil {
		tx.Policies = body.Policies
	}

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privKey", err.Error())
	}

	resp, statusCode, err := c.call("repo_addContributors", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r types.HashResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}
