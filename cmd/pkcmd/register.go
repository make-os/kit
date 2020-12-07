package pkcmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/rpc/types"
	api2 "github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util/api"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
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
	RPCClient types.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce api.NextNonceGetter

	// CreatRegisterPushKey is a function for creating a transaction for registering a push key
	RegisterPushKey api.PushKeyRegister

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
		nextNonce, err := args.GetNextNonce(key.GetUserAddress(), args.RPCClient)
		if err != nil {
			return errors.Wrap(err, "failed to get signer's next nonce")
		}
		nonce, _ = strconv.ParseUint(nextNonce, 10, 64)
	}

	body := &api2.BodyRegisterPushKey{
		PublicKey:  pubKeyToReg,
		Nonce:      nonce,
		Fee:        args.Fee,
		FeeCap:     args.FeeCap,
		Scopes:     args.Scopes,
		SigningKey: key.GetKey(),
	}

	// Create the push key registration transaction
	hash, err := args.RegisterPushKey(body, args.RPCClient)
	if err != nil {
		return errors.Wrap(err, "failed to register push key")
	}

	// Display transaction info and track status
	if args.Stdout != nil {
		ff(args.Stdout, fmt2.NewColor(aurora.Green, aurora.Bold).Sprint("✅ Transaction sent!"))
		ff(args.Stdout, fs(" - Address: %s", fmt2.CyanString("r/"+pubKeyToReg.MustPushKeyAddress().String())))
		ff(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient); err != nil {
			return err
		}
	}

	return nil
}
