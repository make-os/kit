package api

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	rest "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// NextNonceGetter describes a function for getting the next nonce of a the owner of a push key.
type NextNonceGetter func(pushKeyID string, rpcClient client.Client,
	remoteClients []rest.Client) (string, error)

// GetNextNonceOfPushKeyOwner returns the next account nonce of the owner of a given push key.
// It accepts a rpc client and one or more remote API clients represent different remotes.
// If will attempt to first request account information using the remote clients and fallback
// to the RPC client if remote clients failed.
func GetNextNonceOfPushKeyOwner(pkID string, rpcClient client.Client, remoteClients []rest.Client) (string, error) {

	// Return error when no remote client and RPC client were provided
	if len(remoteClients) == 0 && (rpcClient == nil || reflect.ValueOf(rpcClient).IsNil()) {
		return "", fmt.Errorf("no remote client or rpc client provided")
	}

	var err error
	var mainErrs = []error{}

	// If next nonce is not provided, attempt to get the nonce
	// from at least one the remote clients.
	if len(remoteClients) > 0 {
		var resp *types.GetAccountNonceResponse
		var errs []error
		for _, cl := range remoteClients {
			resp, err = cl.GetPushKeyOwnerNonce(pkID)
			if err != nil {
				errs = append(errs, errors.Wrap(err, "Remote API"))
				continue
			}
			break
		}
		if resp != nil {
			curNonce, _ := strconv.ParseUint(resp.Nonce, 10, 64)
			return fmt.Sprintf("%d", curNonce+1), nil
		}
		mainErrs = append(mainErrs, errs...)
	}

	// If an rpc client was provided, attempt to get the nonce using it.
	if rpcClient != nil && !reflect.ValueOf(rpcClient).IsNil() {
		var acct *state.Account
		acct, err = rpcClient.GetPushKeyOwnerAccount(pkID)
		if err != nil {
			mainErrs = append(mainErrs, errors.Wrap(err, "RPC API"))
		}
		if acct != nil {
			return fmt.Sprintf("%d", acct.Nonce+1), nil
		}
	}

	return "", mainErrs[0]
}
