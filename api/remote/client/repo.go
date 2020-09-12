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

// RepoAPI provides access to the repo-related remote APIs.
type RepoAPI struct {
	c *RemoteClient
}

// Create creates transaction to create a new repository
func (c *RepoAPI) Create(body *types.CreateRepoBody) (*types.CreateRepoResponse, error) {

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

	resp, err := c.c.post(V1Path(constants.NamespaceRepo, types.MethodNameCreateRepo), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.CreateRepoResponse
	return &result, resp.ToJSON(&result)
}

// VoteProposal creates transaction to vote for/against a repository's proposal
func (c *RepoAPI) VoteProposal(body *types.RepoVoteBody) (*types.HashResponse, error) {

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

	resp, err := c.c.post(V1Path(constants.NamespaceRepo, types.MethodNameRepoPropVote), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}

// Get returns the repository corresponding to the given name
func (c *RepoAPI) Get(name string, opts ...*types.GetRepoOpts) (*types.GetRepoResponse, error) {

	if len(opts) == 0 {
		opts = []*types.GetRepoOpts{{}}
	}

	path := V1Path(constants.NamespaceRepo, types.MethodNameGetRepo)
	resp, err := c.c.get(path, M{"name": name, "height": opts[0].Height, "noProposals": opts[0].NoProposals})
	if err != nil {
		return nil, err
	}

	var repo = &types.GetRepoResponse{Repository: state.BareRepository()}
	if err = resp.ToJSON(repo.Repository); err != nil {
		return nil, err
	}

	return repo, nil
}

// AddContributors creates transaction to create a add repo contributors
func (c *RepoAPI) AddContributors(body *types.AddRepoContribsBody) (*types.HashResponse, error) {

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

	resp, err := c.c.post(V1Path(constants.NamespaceRepo, types.MethodNameAddRepoContribs), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result types.HashResponse
	return &result, resp.ToJSON(&result)
}
