package client

import (
	"time"

	"gitlab.com/makeos/mosdef/crypto"
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

// CreateRepoResponse is the response object for CreateRepoResponse
type CreateRepoResponse struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

// CreateRepo creates a new repository
func (c *RPCClient) CreateRepo(args CreateRepoArgs) (*CreateRepoResponse, *util.StatusError) {

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
		return nil, util.NewStatusError(400, ErrCodeClient, "privKey", err.Error())
	}

	// call RPC method: repo_create
	resp, statusCode, err := c.call("repo_create", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r CreateRepoResponse
	_ = util.DecodeMap(resp, &r)

	return &r, nil
}

// GetRepoArgs contains arguments for GetRepo
type GetRepoArgs struct {
	Name        string `json:"name"`
	Height      uint64 `json:"height"`
	NoProposals bool   `json:"noProposals"`
}

// GetRepoResponse contains repository information
type GetRepoResponse struct {
	*state.Repository
}

func (c *RPCClient) GetRepo(args *GetRepoArgs) (*GetRepoResponse, *util.StatusError) {

	// call RPC method: repo_get
	resp, statusCode, err := c.call("repo_get", util.StructToMap(args))
	if err != nil {
		return nil, makeStatusErrorFromCallErr(statusCode, err)
	}

	var r = GetRepoResponse{state.BareRepository()}
	if err := util.DecodeMap(resp, r.Repository); err != nil {
		return nil, util.NewStatusError(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
