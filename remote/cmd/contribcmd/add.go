package contribcmd

import (
	"io"
	"strconv"

	"github.com/pkg/errors"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/api/utils"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/cmd"
)

// AddArgs contains arguments for AddCmd.
type AddArgs struct {

	// Name is the unique name of the repository
	Name string

	// Nonce is the next nonce of the signing key's account
	Nonce uint64

	// Fee is the transaction fee to be paid by the signing key
	Fee float64

	// Scopes are the namespaces and repo the contributor
	Scopes []string

	// Account is the account whose key will be used to sign the transaction.
	Account string

	// AccountPass is the passphrase for unlocking the signing key.
	AccountPass string

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker cmd.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce utils.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	CreateRepo utils.RepoCreator

	Stdout io.Writer
}

// AddCmd creates a transaction to add a contributor
func AddCmd(cfg *config.AppConfig, args *AddArgs) error {

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, args.Account, args.AccountPass, nil)
	if err != nil {
		return errors.Wrap(err, "failed to unlock the signing key")
	}

	// If nonce is unset, get the nonce from a remote server
	nonce := args.Nonce
	if nonce == 0 {
		nextNonce, err := args.GetNextNonce(key.GetAddress(), args.RPCClient, args.RemoteClients)
		if err != nil {
			return errors.Wrap(err, "failed to get signer's next nonce")
		}
		nonce, _ = strconv.ParseUint(nextNonce, 10, 64)
	}

	body := &types.RegisterPushKeyBody{
		Nonce:      nonce,
		Fee:        args.Fee,
		SigningKey: key.GetKey(),
	}
	_ = body
	// Create the repo creating transaction
	// hash, err := args.CreateRepo(body, args.RPCClient, args.RemoteClients)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to create repo")
	// }
	//
	// if args.Stdout != nil {
	// fmt.Fprintln(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
	// fmt.Fprintln(args.Stdout, fmt.Sprintf(" - Name: %s", fmt2.CyanString("r/"+body.Name)))
	// fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
	// }

	return nil
}
