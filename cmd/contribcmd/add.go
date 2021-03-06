package contribcmd

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/rpc/types"
	api2 "github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util/api"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

// AddArgs contains arguments for AddCmd.
type AddArgs struct {

	// Name is the name of the repository where the contributors will be added to.
	Name string

	// PushKeys are a list of push keys to add as contributors
	PushKeys []string

	// PropID is the unique proposal ID
	PropID string

	// FeeCap is the contributors' fee cap
	FeeCap float64

	// FeeMode is the contributors' fee mode
	FeeMode int

	// Value is the proposal fee
	Value float64

	// Policies include policies specific to the contributor(s)
	Policies []*state.ContributorPolicy

	// Namespace adds the contributor(s) as namespace-level contributor(s)
	Namespace string

	// NamespaceOnly adds the contributor(s) only as namespace-level contributor(s)
	NamespaceOnly string

	// Nonce is the next nonce of the signing key's account
	Nonce uint64

	// Fee is the transaction fee to be paid by the signing key
	Fee float64

	// SigningKey is the account whose key will be used to sign the transaction.
	SigningKey string

	// AccountPass is the passphrase for unlocking the signing key.
	SigningKeyPass string

	// RpcClient is the RPC client
	RPCClient types.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce api.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	AddRepoContributors api.RepoContributorsAdder

	// ShowTxStatusTracker is a function tracking and displaying tx status
	ShowTxStatusTracker common.TxStatusTrackerFunc

	Stdout io.Writer
}

// AddCmd creates a proposal transaction to add contributors to a repository
func AddCmd(cfg *config.AppConfig, args *AddArgs) error {

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.SigningKeyPass,
		TargetRepo: nil,
		Prompt:     "Enter passphrase to unlock the signing key:\n",
		Stdout:     args.Stdout,
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

	if args.PropID == "" {
		args.PropID = cast.ToString(time.Now().Unix())
	}

	body := &api2.BodyAddRepoContribs{
		RepoName:      args.Name,
		ProposalID:    args.PropID,
		PushKeys:      args.PushKeys,
		FeeCap:        args.FeeCap,
		FeeMode:       args.FeeMode,
		Nonce:         nonce,
		Value:         args.Value,
		Fee:           args.Fee,
		Namespace:     args.Namespace,
		NamespaceOnly: args.NamespaceOnly,
		Policies:      nil,
		SigningKey:    key.GetKey(),
	}

	// Create the repo creating transaction
	hash, err := args.AddRepoContributors(body, args.RPCClient)
	if err != nil {
		return errors.Wrap(err, "failed to add contributors")
	}

	if args.Stdout != nil {
		fmt.Fprintln(args.Stdout, fmt2.NewColor(aurora.Green, aurora.Bold).Sprint("✅ Transaction sent!"))
		fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient); err != nil {
			return err
		}
	}

	return nil
}
