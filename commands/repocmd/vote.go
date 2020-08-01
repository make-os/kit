package repocmd

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
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/config"
	fmt2 "github.com/themakeos/lobe/util/colorfmt"
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
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce utils.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	VoteCreator utils.RepoProposalVoter

	// ShowTxStatusTracker is a function tracking and displaying tx status
	ShowTxStatusTracker common.TxStatusTrackerFunc

	Stdout io.Writer
}

// VoteCmd creates a transaction to vote for/against a repository's proposal
func VoteCmd(cfg *config.AppConfig, args *VoteArgs) error {

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: args.SigningKey,
		Passphrase:   args.SigningKeyPass,
		AskPass:      true,
		TargetRepo:   nil,
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

	body := &types.RepoVoteBody{
		RepoName:   args.RepoName,
		ProposalID: args.ProposalID,
		Vote:       args.Vote,
		Nonce:      nonce,
		Fee:        args.Fee,
		SigningKey: key.GetKey(),
	}

	hash, err := args.VoteCreator(body, args.RPCClient, args.RemoteClients)
	if err != nil {
		return errors.Wrap(err, "failed to cast vote")
	}

	// Display transaction info and track status
	if args.Stdout != nil {
		fmt.Fprintln(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
		fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient, args.RemoteClients); err != nil {
			return err
		}
	}

	return nil
}
