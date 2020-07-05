package client

import (
	"time"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/api"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// CreateRepoArgs contains arguments for CreateRepo
type CreateRepoArgs struct {
	Name       string
	Nonce      uint64
	Value      string
	Fee        string
	Config     *state.RepoConfig
	SigningKey *crypto.Key
}

// CreateRepo creates a new repository
func (c *RPCClient) CreateRepo(args *CreateRepoArgs) (*api.CreateRepoResponse, *util.StatusError) {

	// Create a TxRepoCreate object and fill it with args
	tx := txns.NewBareTxRepoCreate()
	tx.Name = args.Name
	tx.Nonce = args.Nonce
	tx.Value = util.String(args.Value)
	tx.Fee = util.String(args.Fee)
	tx.Timestamp = time.Now().Unix()
	tx.Config = args.Config.ToMap()
	tx.SenderPubKey = args.SigningKey.PubKey().ToPublicKey()

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(args.SigningKey.PrivKey().Base58())
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

// GetRepoOpts contains arguments for GetRepo
type GetRepoOpts struct {
	Height      uint64 `json:"height"`
	NoProposals bool   `json:"noProposals"`
}

// GetRepo finds and returns a repository
func (c *RPCClient) GetRepo(name string, opts ...*GetRepoOpts) (*api.GetRepoResponse, *util.StatusError) {

	if len(opts) == 0 {
		opts = []*GetRepoOpts{{}}
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
