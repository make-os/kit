package client

import (
	"fmt"
	"time"

	"github.com/spf13/cast"
	"gitlab.com/makeos/mosdef/api/remote"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// CreateRepo creates transaction to create a new repository
func (c *ClientV1) CreateRepo(body *types.CreateRepoBody) (*types.CreateRepoResponse, error) {

	if body.SigningKey == nil {
		return nil, fmt.Errorf("signing key is required")
	}

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
		return nil, err
	}

	resp, err := c.post(remote.V1Path(constants.NamespaceRepo, types.MethodNameCreateRepo), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.CreateRepoResponse
	return &result, resp.ToJSON(&result)
}

// VoteRepoProposal creates transaction to vote for/against a repository's proposal
func (c *ClientV1) VoteRepoProposal(body *types.RepoVoteBody) (*types.HashResponse, error) {

	if body.SigningKey == nil {
		return nil, fmt.Errorf("signing key is required")
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
		return nil, err
	}

	resp, err := c.post(remote.V1Path(constants.NamespaceRepo, types.MethodNameRepoPropVote), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}

// GetRepo returns the repository corresponding to the given name
func (c *ClientV1) GetRepo(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error) {

	if len(opts) == 0 {
		opts = []*types.GetRepoOpts{{}}
	}

	path := remote.V1Path(constants.NamespaceRepo, types.MethodNameGetRepo)
	resp, err := c.get(path, M{"name": name, "height": opts[0].Height, "noProposals": opts[0].NoProposals})
	if err != nil {
		return nil, err
	}

	var repo = &types.GetRepoResponse{Repository: state.BareRepository()}
	if err = resp.ToJSON(repo.Repository); err != nil {
		return nil, err
	}

	return repo, nil
}

// AddRepoContributors creates transaction to create a add repo contributors
func (c *ClientV1) AddRepoContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error) {

	if body.SigningKey == nil {
		return nil, fmt.Errorf("signing key is required")
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
		return nil, err
	}

	resp, err := c.post(remote.V1Path(constants.NamespaceRepo, types.MethodNameAddRepoContribs), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}
