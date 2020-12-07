package client

import (
	"time"

	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
)

// RepoAPI provides access to the repo-related RPC methods
type RepoAPI struct {
	c *RPCClient
}

// Create creates a new repository
func (c *RepoAPI) Create(body *api.BodyCreateRepo) (*api.ResultCreateRepo, error) {

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
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, statusCode, err := c.c.call("repo_create", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r api.ResultCreateRepo
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// Get finds and returns a repository
func (c *RepoAPI) Get(name string, opts ...*api.GetRepoOpts) (*api.ResultRepository, error) {

	if len(opts) == 0 {
		opts = []*api.GetRepoOpts{{}}
	}

	params := util.Map{"name": name, "height": opts[0].Height, "noProposals": opts[0].NoProposals}
	resp, statusCode, err := c.c.call("repo_get", params)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = api.ResultRepository{Repository: state.BareRepository()}
	if err := util.DecodeMap(resp, r.Repository); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// AddContributors creates transaction to create a add repo contributors
func (c *RepoAPI) AddContributors(body *api.BodyAddRepoContribs) (*api.ResultHash, error) {

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
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, statusCode, err := c.c.call("repo_addContributor", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r api.ResultHash
	if err := util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// VoteProposal creates transaction to vote for/against a repository's proposal
func (c *RepoAPI) VoteProposal(body *api.BodyRepoVote) (*api.ResultHash, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareRepoProposalVote()
	tx.RepoName = body.RepoName
	tx.ProposalID = body.ProposalID
	tx.Vote = body.Vote
	tx.Nonce = body.Nonce
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()

	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, statusCode, err := c.c.call("repo_vote", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r api.ResultHash
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
