package utils

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	remote "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/types"
)

// NextNonceGetter describes a function for getting the next nonce of an account.
type NextNonceGetter func(
	address string,
	rpcClient client.Client,
	remoteClients []remote.Client) (string, error)

// GetNextNonceOfPushKeyOwner returns the next account nonce of the owner of a given push key.
func GetNextNonceOfPushKeyOwner(
	pkID string,
	rpcClient client.Client,
	remoteClients []remote.Client) (nextNonce string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		var err error
		var acct *types.GetAccountResponse
		acct, err = c.GetPushKeyOwner(pkID)
		if acct != nil {
			nextNonce = fmt.Sprintf("%d", acct.Nonce+1)
		}
		return err

	}, func(c remote.Client) error {
		var err error
		var resp *types.GetAccountNonceResponse
		resp, err = c.GetPushKeyOwnerNonce(pkID)
		if resp != nil {
			curNonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
			nextNonce = fmt.Sprintf("%d", curNonce+1)
		}
		return err
	})
	return
}

// GetNextNonceOfAccount returns the next account nonce of an account.
func GetNextNonceOfAccount(
	address string,
	rpcClient client.Client,
	remoteClients []remote.Client) (nextNonce string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		var err error
		var acct *types.GetAccountResponse
		acct, err = c.GetAccount(address)
		if acct != nil {
			nextNonce = fmt.Sprintf("%d", acct.Nonce+1)
		}
		return err

	}, func(c remote.Client) error {
		var err error
		var resp *types.GetAccountResponse
		resp, err = c.GetAccount(address)
		if resp != nil {
			nextNonce = fmt.Sprintf("%d", resp.Nonce.UInt64()+1)
		}
		return err
	})
	return
}

// RepoCreator describes a function for creating a repo creating transaction.
type RepoCreator func(
	req *types.CreateRepoBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error)

// CreateRepo creates a repository creating transaction and returns the hash.
func CreateRepo(
	req *types.CreateRepoBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		resp, err := c.CreateRepo(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err

	}, func(c remote.Client) error {
		resp, err := c.CreateRepo(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err
	})
	return
}

// RepoCreator describes a function for creating a repo creating transaction.
type PushKeyRegister func(
	req *types.RegisterPushKeyBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error)

// RegisterPushKey creates a push key registration transaction and returns the hash.
func RegisterPushKey(
	req *types.RegisterPushKeyBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		resp, err := c.RegisterPushKey(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err

	}, func(c remote.Client) error {
		resp, err := c.RegisterPushKey(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err
	})
	return
}

// RepoContributorsAdder describes a function for creating a
// proposal to add contributors to a repo.
type RepoContributorsAdder func(
	req *types.AddRepoContribsBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error)

// AddRepoContributors creates a proposal transaction to add contributors
// to a repo and returns the hash.
func AddRepoContributors(
	req *types.AddRepoContribsBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		resp, err := c.AddRepoContributors(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err

	}, func(c remote.Client) error {
		resp, err := c.AddRepoContributors(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err
	})
	return
}

// CoinSender describes a function for sending coins
type CoinSender func(
	req *types.SendCoinBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error)

// SendCoin creates a transaction to send coins from user account to
// another user/repo account.
func SendCoin(
	req *types.SendCoinBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		resp, err := c.SendCoin(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err

	}, func(c remote.Client) error {
		resp, err := c.SendCoin(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err
	})
	return
}

// TxGetter describes a function for getting a finalized transaction
type TxGetter func(
	hash string,
	rpcClient client.Client,
	remoteClients []remote.Client) (res *types.GetTxResponse, err error)

// GetTransaction gets a finalized transaction by hash
func GetTransaction(
	hash string,
	rpcClient client.Client,
	remoteClients []remote.Client) (res *types.GetTxResponse, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		resp, err := c.GetTransaction(hash)
		if err != nil {
			return err
		}
		res = resp
		return err

	}, func(c remote.Client) error {
		resp, err := c.GetTransaction(hash)
		if err != nil {
			return err
		}
		res = resp
		return err
	})
	return
}

// RepoProposalVoter describes a function for voting on a repo's proposal
type RepoProposalVoter func(
	req *types.RepoVoteBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error)

// VoteRepoProposal creates a transaction to vote for/on a repo's proposal
func VoteRepoProposal(
	req *types.RepoVoteBody,
	rpcClient client.Client,
	remoteClients []remote.Client) (hash string, err error) {
	err = CallClients(rpcClient, remoteClients, func(c client.Client) error {
		resp, err := c.VoteRepoProposal(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err

	}, func(c remote.Client) error {
		resp, err := c.VoteRepoProposal(req)
		if err != nil {
			return err
		}
		hash = resp.Hash
		return err
	})
	return
}

// CallClients allows the caller to perform calls on multiple remote clients
// and an RPC client. Callers must provide rpcCaller callback function to perform
// operation with the given rpc client.
//
// Likewise, caller must provide remoteCaller callback function to perform operation
// with the given remote API clients.
//
// No further calls to remote API clients will occur once nil is returned from one
// of the clients. The rpcClient is not called when one of the remote API client
// returns a nil error.
func CallClients(
	rpcClient client.Client,
	remoteClients []remote.Client,
	rpcCaller func(client.Client) error,
	remoteCaller func(remote.Client) error) error {

	// Return error when no remote client and RPC client were provided
	if len(remoteClients) == 0 && (rpcClient == nil || reflect.ValueOf(rpcClient).IsNil()) {
		return fmt.Errorf("no remote client or rpc client provided")
	}

	// Return error if either rpc caller or remote caller was not provided.
	if rpcCaller == nil && remoteCaller == nil {
		return fmt.Errorf("no client caller provided")
	}

	var err error
	var mainErrs = []error{}

	// If remote clients are provided, call each one with the remote API caller
	// passing the client to the callback function.
	// Stop calling for each client once one succeeds.
	if len(remoteClients) > 0 {
		var errs []error
		for _, cl := range remoteClients {
			if remoteCaller != nil {
				err = remoteCaller(cl)
				if err != nil {
					errs = append(errs, errors.Wrap(err, "Remote API"))
					continue
				}
			}
			break
		}
		mainErrs = append(mainErrs, errs...)

		// Return nil immediately if not all remote API clients failed
		if len(mainErrs) < len(remoteClients) {
			return nil
		}
	}

	// If an rpc client was provided, attempt to call the rpc client caller with it.
	if rpcClient != nil && !reflect.ValueOf(rpcClient).IsNil() {
		if rpcCaller != nil {
			err = rpcCaller(rpcClient)
			if err != nil && !reflect.ValueOf(err).IsNil() {
				mainErrs = append(mainErrs, errors.Wrap(err, "RPC API"))
			}
		}
	}

	// Return nil immediately if no error
	if len(mainErrs) == 0 {
		return nil
	}

	return mainErrs[0]
}
