package repocmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/rpc/types"
	api2 "github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util/api"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
)

// VoteArgs contains arguments for VoteCmd.
type VoteArgs struct {

	// Name is the name of the repository
	RepoName string

	// ProposalID is the unique ID of the proposal
	ProposalID string

	// Vote is the vote choice
	Vote int

	// Nonce is the next nonce of the signing key's account
	Nonce uint64

	// Fee is the transaction fee to be paid by the signing key
	Fee float64

	// SigningKey is the account whose key will be used to sign the transaction.
	SigningKey string

	// SigningKeyPass is the passphrase for unlocking the signing key.
	SigningKeyPass string

	// RpcClient is the RPC client
	RPCClient types.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce api.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	VoteCreator api.RepoProposalVoter

	// ShowTxStatusTracker is a function tracking and displaying tx status
	ShowTxStatusTracker common.TxStatusTrackerFunc

	Stdout io.Writer
}

// VoteCmd creates a transaction to vote for/against a repository's proposal
func VoteCmd(cfg *config.AppConfig, args *VoteArgs) error {

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
		nextNonce, err := args.GetNextNonce(key.GetUserAddress(), args.RPCClient)
		if err != nil {
			return errors.Wrap(err, "failed to get signer's next nonce")
		}
		nonce, _ = strconv.ParseUint(nextNonce, 10, 64)
	}

	body := &api2.BodyRepoVote{
		RepoName:   args.RepoName,
		ProposalID: args.ProposalID,
		Vote:       args.Vote,
		Nonce:      nonce,
		Fee:        args.Fee,
		SigningKey: key.GetKey(),
	}

	hash, err := args.VoteCreator(body, args.RPCClient)
	if err != nil {
		return errors.Wrap(err, "failed to cast vote")
	}

	// Display transaction info and track status
	if args.Stdout != nil {
		fmt.Fprintln(args.Stdout, fmt2.NewColor(aurora.Green, aurora.Bold).Sprint("✅ Transaction sent!"))
		fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient); err != nil {
			return err
		}
	}

	return nil
}
