package client

import (
	"fmt"
	"time"

	"gitlab.com/makeos/mosdef/remote/api"
	apitypes "gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// CreateAccount returns the nonce of the given address
func (c *ClientV1) CreateRepo(body *apitypes.CreateRepoBody) (*apitypes.CreateRepoResponse, error) {

	if body.SigningKey == nil {
		return nil, fmt.Errorf("signing key is required")
	}

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
		return nil, err
	}

	resp, err := c.post(api.V1Path(constants.NamespaceRepo, apitypes.MethodNameCreateRepo), tx.ToMap())
	if err != nil {
		return nil, err
	}

	var result apitypes.CreateRepoResponse
	return &result, resp.ToJSON(&result)
}

// GetRepo returns the repository corresponding to the given name
func (c *ClientV1) GetRepo(name string, opts ...*apitypes.GetRepoOpts) (*apitypes.GetRepoResponse, error) {

	if len(opts) == 0 {
		opts = []*apitypes.GetRepoOpts{{}}
	}

	path := api.V1Path(constants.NamespaceRepo, apitypes.MethodNameGetRepo)
	resp, err := c.get(path, M{"name": name, "height": opts[0].Height, "noProposals": opts[0].NoProposals})
	if err != nil {
		return nil, err
	}

	var repo = &apitypes.GetRepoResponse{Repository: state.BareRepository()}
	if err = resp.ToJSON(repo.Repository); err != nil {
		return nil, err
	}

	return repo, nil
}
