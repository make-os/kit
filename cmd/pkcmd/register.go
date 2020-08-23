package pkcmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/types"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	fmt2 "github.com/themakeos/lobe/util/colorfmt"
)

var (
	fs = fmt.Sprintf
	ff = fmt.Fprintln
)

// RegisterArgs contains arguments for RegisterCmd.
type RegisterArgs struct {

	// PublicKeyOrAddressID refers the the key to be registered. It may contain a base58
	// public key or an address/index of a local account
	Target string

	// TargetPass contains the passphrase for unlocking the target if it refers to a local account
	TargetPass string

	// Nonce is the next nonce of the signing key's account
	Nonce uint64

	// Fee is the transaction fee to be paid by the signing key
	Fee float64

	// Scopes are the namespaces and repo the contributor
	Scopes []string

	// FeeCap is the hard limit of how much fees spendable by the push key
	FeeCap float64

	// Account is the account whose key will be used to sign the transaction.
	SigningKey string

	// AccountPass is the passphrase for unlocking the signing key.
	SigningKeyPass string

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce utils.NextNonceGetter

	// CreatRegisterPushKey is a function for creating a transaction for registering a push key
	RegisterPushKey utils.PushKeyRegister

	// ShowTxStatusTracker is a function tracking and displaying tx status
	ShowTxStatusTracker common.TxStatusTrackerFunc

	Stdout io.Writer
}

// RegisterCmd creates a transaction to register a public key as a push key
func RegisterCmd(cfg *config.AppConfig, args *RegisterArgs) error {

	var pubKeyToReg crypto.PublicKey

	// Check if target to register is a valid public key.
	// If not, we assume it is an ID of a local account and attempt to unlock it.
	if crypto.IsValidPubKey(args.Target) == nil {
		pk, _ := crypto.PubKeyFromBase58(args.Target)
		pubKeyToReg = pk.ToPublicKey()
	} else {
		key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
			KeyStoreID: args.Target,
			Passphrase: args.TargetPass,
			TargetRepo: nil,
			Prompt:     "Enter passphrase to unlock the target key:\n",
		})
		if err != nil {
			return errors.Wrap(err, "failed to unlock the local key")
		}
		pubKeyToReg = key.GetKey().PubKey().ToPublicKey()
	}

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.SigningKeyPass,
		TargetRepo: nil,
		Prompt:     "Enter passphrase to unlock the signing key:\n",
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

	body := &types.RegisterPushKeyBody{
		PublicKey:  pubKeyToReg,
		Nonce:      nonce,
		Fee:        args.Fee,
		FeeCap:     args.FeeCap,
		Scopes:     args.Scopes,
		SigningKey: key.GetKey(),
	}

	// Create the push key registration transaction
	hash, err := args.RegisterPushKey(body, args.RPCClient, args.RemoteClients)
	if err != nil {
		return errors.Wrap(err, "failed to register push key")
	}

	// Display transaction info and track status
	if args.Stdout != nil {
		ff(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
		ff(args.Stdout, fs(" - Address: %s", fmt2.CyanString("r/"+pubKeyToReg.MustPushKeyAddress().String())))
		ff(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient, args.RemoteClients); err != nil {
			return err
		}
	}

	return nil
}
