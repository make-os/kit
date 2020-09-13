package repocmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/fatih/color"
	restclient "github.com/make-os/lobe/api/remote/client"
	"github.com/make-os/lobe/api/rpc/client"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/api/utils"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	fmt2 "github.com/make-os/lobe/util/colorfmt"
	"github.com/pkg/errors"
	"github.com/stretchr/objx"
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
	Value float64

	// Fee is the transaction fee to be paid by the signing key
	Fee float64

	// SigningKey is the account whose key will be used to sign the transaction.
	SigningKey string

	// SigningKeyPass is the passphrase for unlocking the signing key.
	SigningKeyPass string

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce utils.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	CreateRepo utils.RepoCreator

	// ShowTxStatusTracker is a function tracking and displaying tx status
	ShowTxStatusTracker common.TxStatusTrackerFunc

	Stdout io.Writer
}

// CreateCmd creates a transaction to create a repository
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
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.SigningKeyPass,
		TargetRepo: nil,
	})
	if err != nil {
		return errors.Wrap(err, "failed to unlock the signing key")
	}

	// If nonce is unset, get the nonce from a remote server
	nonce := args.Nonce
	if nonce == 0 {
		nextNonce, err := args.GetNextNonce(key.GetUserAddress(), args.RPCClient, args.RemoteClients)
		if err != nil {
			return errors.Wrap(err, "failed to get signer's next nonce")
		}
		nonce, _ = strconv.ParseUint(nextNonce, 10, 64)
	}

	body := &types.BodyCreateRepo{
		Name:       args.Name,
		Nonce:      nonce,
		Value:      args.Value,
		Fee:        args.Fee,
		Config:     repoCfg,
		SigningKey: key.GetKey(),
	}

	// Create the repo creating transaction
	hash, err := args.CreateRepo(body, args.RPCClient, args.RemoteClients)
	if err != nil {
		return errors.Wrap(err, "failed to create repo")
	}

	// Display transaction info and track status
	if args.Stdout != nil {
		fmt.Fprintln(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
		fmt.Fprintln(args.Stdout, fmt.Sprintf(" - Name: %s", fmt2.CyanString("r/"+body.Name)))
		fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient, args.RemoteClients); err != nil {
			return err
		}
	}

	return nil
}
