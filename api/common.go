package api

import (
	"fmt"

	"github.com/pkg/errors"
	client2 "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/util"
)

// DetermineNextNonceOfGPGKeyOwner is used to determine the next nonce of the account that
// owns the given gpg key ID.
//
// It will use the rpc and remote clients as source to request for
// the current account nonce.
//
// First, it will query remote API clients and will use the first value returned by
// any of the clients, increment the value by 1 and return it as the next nonce.
//
// If still unable to get the nonce, it will attempt to query the JSON-RPC client
// client, increment its result by 1 and return it as the next nonce.
//
// It returns error if unable to get nonce.
func DetermineNextNonceOfGPGKeyOwner(
	gpgID string,
	rpcClient *client.RPCClient,
	remoteClients []*client2.RESTClient) (string, error) {

	var nonce string

	// If nonce is not provided, attempt to get the nonce from the remote API.
	var errRemoteClients error
	if util.IsZeroString(nonce) && len(remoteClients) > 0 {
		nonce, errRemoteClients = client2.GPGGetNextNonceOfOwnerUsingClients(remoteClients, gpgID)
		errRemoteClients = errors.Wrap(errRemoteClients, "remote-client")
	}

	// If the nonce is still not known and rpc client non-nil, attempt to get nonce using the client
	var errRPCClient error
	if util.IsZeroString(nonce) && rpcClient != nil {
		nonce, errRPCClient = client.GPGGetNonceOfOwnerUsingRPCClient(gpgID, rpcClient)
		errRPCClient = errors.Wrap(errRPCClient, "rpc-client")
	}

	// Check errors and return appropriate error messages
	if errRemoteClients != nil && errRPCClient != nil {
		wrapped := errors.Wrap(errRemoteClients, errRPCClient.Error())
		msg := "failed to get nonce using both Remote API and JSON-RPC 2.0 API clients"
		return "", errors.Wrap(wrapped, msg)
	} else if errRemoteClients != nil {
		msg := "failed to get nonce using Remote API client"
		return "", errors.Wrap(errRemoteClients, msg)
	} else if errRPCClient != nil {
		msg := "failed to get nonce using JSON-RPC 2.0 API client"
		return "", errors.Wrap(errRPCClient, msg)
	}

	if util.IsZeroString(nonce) {
		return "", fmt.Errorf("signer's account nonce is required")
	}

	return nonce, nil
}

// First, it will query remote API clients and will use the first value returned by
// any of the clients, increment the value by 1 and return it as the next nonce.
//
// If still unable to get the nonce, it will attempt to query the JSON-RPC
// client, increment its result by 1 and return it as the next nonce.
//
// It returns error if unable to get nonce.
func DetermineNextNonceOfAccount(
	address string,
	rpcClient *client.RPCClient,
	remoteClients []*client2.RESTClient) (string, error) {

	var nonce string

	// If nonce is not provided, attempt to get the nonce from the remote API.
	var errRemoteClients error
	if util.IsZeroString(nonce) && len(remoteClients) > 0 {
		nonce, errRemoteClients = client2.AccountGetNextNonceUsingClients(remoteClients, address)
		errRemoteClients = errors.Wrap(errRemoteClients, "remote-client")
	}

	// If the nonce is still not known and rpc client non-nil, attempt to get nonce using the client
	var errRPCClient error
	if util.IsZeroString(nonce) && rpcClient != nil {
		nonce, errRPCClient = client.AccountGetNextNonceUsingRPCClient(address, rpcClient)
		errRPCClient = errors.Wrap(errRPCClient, "rpc-client")
	}

	// Check errors and return appropriate error messages
	if errRemoteClients != nil && errRPCClient != nil {
		wrapped := errors.Wrap(errRemoteClients, errRPCClient.Error())
		msg := "failed to get nonce using both Remote API and JSON-RPC 2.0 API clients"
		return "", errors.Wrap(wrapped, msg)
	} else if errRemoteClients != nil {
		msg := "failed to get nonce using Remote API client"
		return "", errors.Wrap(errRemoteClients, msg)
	} else if errRPCClient != nil {
		msg := "failed to get nonce using JSON-RPC 2.0 API client"
		return "", errors.Wrap(errRPCClient, msg)
	}

	if util.IsZeroString(nonce) {
		return "", fmt.Errorf("signer's account nonce is required")
	}

	return nonce, nil
}
