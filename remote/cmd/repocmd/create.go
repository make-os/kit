package repocmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/stretchr/objx"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/api/utils"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/cmd"
	"gitlab.com/makeos/mosdef/types/state"
	fmt2 "gitlab.com/makeos/mosdef/util/fmt"
)

// CreateArgs contains arguments for CreateCmd.
type CreateArgs struct {

	// Name is the unique name of the repository
	Name string

	// configPath is the path to the repo config file or a JSON string
	// to be decoded as the config.
	Config string

	// Nonce is the next nonce of the signing key's account
	Nonce uint64

	// Value the the amount of coins to transfer from the signer's account to the repo account.
	Value string

	// Fee is the transaction fee to be paid by the signing key
	Fee string

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

// CreateCmd creates a repository
func CreateCmd(cfg *config.AppConfig, args *CreateArgs) error {

	// If path is set, read repo config from file.
	// If path is JSON, parse it as the config.
	var repoCfg map[string]interface{}
	if args.Config != "" {
		var bz []byte
		var err error
		var path = args.Config

		if govalidator.IsJSON(path) {
			bz = []byte(path)
			goto parse
		}

		path, _ = filepath.Abs(path)
		bz, err = ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrap(err, "failed to read config file")
		}

	parse:
		repoCfg, err = objx.FromJSON(string(bz))
		if err != nil {
			return errors.Wrap(err, "failed parse configuration")
		}
	}

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

	body := &types.CreateRepoBody{
		Name:       args.Name,
		Nonce:      nonce,
		Value:      args.Value,
		Fee:        args.Fee,
		SigningKey: key.GetKey(),
	}

	if len(repoCfg) > 0 {
		body.Config = state.NewDefaultRepoConfigFromMap(repoCfg)
	}

	// Create the repo creating transaction
	hash, err := args.CreateRepo(body, args.RPCClient, args.RemoteClients)
	if err != nil {
		return errors.Wrap(err, "failed to create repo")
	}

	if args.Stdout == nil {
		args.Stdout = ioutil.Discard
	}
	fmt.Fprintln(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
	fmt.Fprintln(args.Stdout, fmt.Sprintf(" - Name: %s", fmt2.CyanString("r/"+body.Name)))
	fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))

	return nil
}
