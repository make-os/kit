package clients

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	rest "gitlab.com/makeos/mosdef/remote/api/client"
	"gitlab.com/makeos/mosdef/rpc/api/client"
	"gitlab.com/makeos/mosdef/types/api"
)

// NextNonceGetter describes a function for getting the next nonce of a the owner of a push key.
type NextNonceGetter func(pushKeyID string, rpcClient client.Client,
	remoteClients []rest.Client) (string, error)

// GetNextNonceOfPushKeyOwner returns the next account nonce of the owner of a given push key.
// It accepts a rpc client and one or more remote API clients represent different remotes.
// If will attempt to first request account information using the remote clients and fallback
// to the RPC client if remote clients fail.
func GetNextNonceOfPushKeyOwner(pkID string, rpcClient client.Client, remoteClients []rest.Client) (string, error) {

	var nextNonce string
	err := CallClients(rpcClient, remoteClients, func(c client.Client) error {
		var err error
		var acct *api.GetAccountResponse

		acct, err = rpcClient.GetPushKeyOwner(pkID)
		if acct != nil {
			nextNonce = fmt.Sprintf("%d", acct.Nonce+1)
		}

		return err

	}, func(cl rest.Client) error {
		var err error
		var resp *api.GetAccountNonceResponse

		resp, err = cl.GetPushKeyOwnerNonce(pkID)
		if resp != nil {
			curNonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
			nextNonce = fmt.Sprintf("%d", curNonce+1)
		}

		return err
	})

	return nextNonce, err
}

// CallClients allows the caller to perform calls on multiple remote clients
// and an RPC client. Callers must provide rpcCaller callback function to perform
// operation with the given rpc client.
//
// Likewise, caller must provide remoteCaller callback function to perform operation
// with the given remote API clients.
//
// No further calls to remote API clients will occur once nil is
// returned from one of the clients.
//
// The rpcClient is not called when one of the remote API client
// returns a nil error.
func CallClients(
	rpcClient client.Client,
	remoteClients []rest.Client,
	rpcCaller func(client.Client) error,
	remoteCaller func(rest.Client) error) error {

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
