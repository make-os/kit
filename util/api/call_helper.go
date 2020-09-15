package api

import (
	"fmt"

	"github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/types/api"
)

// NextNonceGetter describes a function for getting the next nonce of an account.
type NextNonceGetter func(address string, c types.Client) (string, error)

// GetNextNonceOfPushKeyOwner returns the next account nonce of the owner of a given push key.
func GetNextNonceOfPushKeyOwner(pkID string, c types.Client) (nextNonce string, err error) {
	var acct *api.ResultAccount
	acct, err = c.PushKey().GetOwner(pkID)
	if acct != nil {
		nextNonce = fmt.Sprintf("%d", acct.Nonce+1)
	}
	return
}

// GetNextNonceOfAccount returns the next account nonce of an account.
func GetNextNonceOfAccount(address string, c types.Client) (nextNonce string, err error) {
	var acct *api.ResultAccount
	acct, err = c.User().Get(address)
	if acct != nil {
		nextNonce = fmt.Sprintf("%d", acct.Nonce+1)
	}
	return
}

// RepoCreator describes a function for creating a repo creating transaction.
type RepoCreator func(req *api.BodyCreateRepo, c types.Client) (hash string, err error)

// CreateRepo creates a repository creating transaction and returns the hash.
func CreateRepo(req *api.BodyCreateRepo, c types.Client) (hash string, err error) {
	resp, err := c.Repo().Create(req)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// RepoCreator describes a function for creating a repo creating transaction.
type PushKeyRegister func(req *api.BodyRegisterPushKey, c types.Client) (hash string, err error)

// RegisterPushKey creates a push key registration transaction and returns the hash.
func RegisterPushKey(req *api.BodyRegisterPushKey, c types.Client) (hash string, err error) {
	resp, err := c.PushKey().Register(req)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// RepoContributorsAdder describes a function for creating a
// proposal to add contributors to a repo.
type RepoContributorsAdder func(
	req *api.BodyAddRepoContribs,
	c types.Client) (hash string, err error)

// AddRepoContributors creates a proposal transaction to add contributors
// to a repo and returns the hash.
func AddRepoContributors(req *api.BodyAddRepoContribs, c types.Client) (hash string, err error) {
	resp, err := c.Repo().AddContributors(req)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// CoinSender describes a function for sending coins
type CoinSender func(req *api.BodySendCoin, c types.Client) (hash string, err error)

// SendCoin creates a transaction to send coins from user account to
// another user/repo account.
func SendCoin(req *api.BodySendCoin, c types.Client) (hash string, err error) {
	resp, err := c.User().Send(req)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// TxGetter describes a function for getting a finalized transaction
type TxGetter func(
	hash string,
	c types.Client) (res *api.ResultTx, err error)

// GetTransaction gets a finalized transaction by hash
func GetTransaction(hash string, c types.Client) (res *api.ResultTx, err error) {
	return c.Tx().Get(hash)
}

// RepoProposalVoter describes a function for voting on a repo's proposal
type RepoProposalVoter func(req *api.BodyRepoVote, c types.Client) (hash string, err error)

// VoteRepoProposal creates a transaction to vote for/on a repo's proposal
func VoteRepoProposal(req *api.BodyRepoVote, c types.Client) (hash string, err error) {
	resp, err := c.Repo().VoteProposal(req)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}
