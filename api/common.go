package api

import (
	"fmt"

	"github.com/pkg/errors"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/util"
)

// NextNonceGetter describes a function for getting next nonce of a pusher
// using the remote rest client or the json rpc client
type NextNonceGetter func(
	pushKeyID string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) (string, error)

// GetNextNonceOfPushKeyOwner is used to determine the next nonce of the account that
// owns the target push key ID.
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
func GetNextNonceOfPushKeyOwner(
	pushKeyID string,
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) (string, error) {

	var nextNonce string

	// If next nonce is not provided, attempt to get the nonce from the remote API.
	var errRemote error
	if len(remoteClients) > 0 {
		nextNonce, errRemote = restclient.PushKeyGetNextNonceOfOwnerUsingClients(remoteClients, pushKeyID)
	}

	// If the nonce is still not known and rpc client non-nil, attempt to get nonce using the client
	var errRPC error
	if util.IsZeroString(nextNonce) && rpcClient != nil {
		nextNonce, errRPC = client.GetNextNonceOfPushKeyOwnerUsingRPCClient(pushKeyID, rpcClient)
	}

	// At this point, we have failed to use the clients to get the next nonce.
	// Return appropriate error messages
	errRemote = errors.Wrap(errRemote, "remote")
	errRPC = errors.Wrap(errRPC, "rpc")
	if errRemote != nil && errRPC != nil {
		wrapped := errors.Wrap(errRemote, errRPC.Error())
		msg := "failed to request next nonce using remote or rpc clients"
		return "", errors.Wrap(wrapped, msg)
	} else if errRemote != nil {
		msg := "failed to request next nonce using remote client"
		return "", errors.Wrap(errRemote, msg)
	} else if errRPC != nil {
		msg := "failed to request next nonce using rpc client"
		return "", errors.Wrap(errRPC, msg)
	}

	if util.IsZeroString(nextNonce) {
		return "", fmt.Errorf("signer's account nonce is required")
	}

	return nextNonce, nil
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
	remoteClients []restclient.RestClient) (string, error) {

	var nextNonce string

	// If there is at least 1 Remote API client, attempt to request the next nextNonce
	var errRemote error
	if len(remoteClients) > 0 {
		nextNonce, errRemote = restclient.AccountGetNextNonceUsingClients(remoteClients, address)
	}

	// If the nextNonce is still not known and rpc client non-nil, attempt to get nextNonce using the client
	var errRPC error
	if util.IsZeroString(nextNonce) && rpcClient != nil {
		nextNonce, errRPC = client.GetNextNonceOfAccountUsingRPCClient(address, rpcClient)
	}

	// At this point, we have failed to use the clients to get the next nonce.
	// Return appropriate error messages
	errRemote = errors.Wrap(errRemote, "remote")
	errRPC = errors.Wrap(errRPC, "rpc")
	if errRemote != nil && errRPC != nil {
		wrapped := errors.Wrap(errRemote, errRPC.Error())
		msg := "failed to request next nonce using remote or rpc clients"
		return "", errors.Wrap(wrapped, msg)
	} else if errRemote != nil {
		msg := "failed to request next nonce using remote client"
		return "", errors.Wrap(errRemote, msg)
	} else if errRPC != nil {
		msg := "failed to request next nonce using rpc client"
		return "", errors.Wrap(errRPC, msg)
	}

	if util.IsZeroString(nextNonce) {
		return "", fmt.Errorf("signer's account nextNonce is required")
	}

	return nextNonce, nil
}

// SendTxPayload sends a signed transaction payload to the mempool using either
// an JSONRPC 2.0 client or one of several Remote API clients.
func SendTxPayload(
	data map[string]interface{},
	rpcClient *client.RPCClient,
	remoteClients []restclient.RestClient) (string, error) {

	var resp *types.TxSendPayloadResponse
	var errRPC, errRemote error

	// If at least 1 remote client is provided, try to use the best one to send the transaction payload.
	// Immediately return error if a non-500 http code is returned.
	if len(remoteClients) > 0 {
		resp, errRemote = restclient.TxSendPayloadUsingClients(remoteClients, data)
		if errRemote != nil {
			statusErr := util.StatusErrorFromStr(errRemote.Error())
			if statusErr.HttpCode > 0 && statusErr.HttpCode != 500 {
				return "", fmt.Errorf(statusErr.Msg)
			}
		}
		if resp != nil {
			return resp.Hash, nil
		}
	}

	// If an rpc client is provided, attempt to use it to send the transaction payload
	// Immediately return error if a non-500 http code is returned.
	if rpcClient != nil {
		resp, errRPC = rpcClient.TxSendPayload(data)
		if errRPC != nil {
			statusErr := util.StatusErrorFromStr(errRPC.Error())
			if statusErr.HttpCode > 0 && statusErr.HttpCode != 500 {
				return "", fmt.Errorf(statusErr.Msg)
			}
		}
		if resp != nil {
			return resp.Hash, nil
		}
	}

	// At this point, all attempts to send the payload using the clients have failed.
	// Return appropriate error messages.
	errRemote = errors.Wrap(errRemote, "remote")
	errRPC = errors.Wrap(errRPC, "rpc")
	if errRemote != nil && errRPC != nil {
		msg := "failed to send request using remote or rpc clients"
		return "", errors.Wrap(errors.Wrap(errRemote, errRPC.Error()), msg)
	} else if errRemote != nil {
		msg := "failed to send request using Remote API client"
		return "", errors.Wrap(errRemote, msg)
	} else if errRPC != nil {
		msg := "failed to send request using RPC client"
		return "", errors.Wrap(errRPC, msg)
	}

	return "", fmt.Errorf("failed")
}
